// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package sandbox provides agent-sandbox-based sandboxed command execution
// using the Kubernetes agent-sandbox CRD (agents.x-k8s.io/v1alpha1)
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// AgentSandbox represents an agent-sandbox-based execution environment
// It uses the Kubernetes agent-sandbox CRD (agents.x-k8s.io/v1alpha1)
type AgentSandbox struct {
	name          string
	namespace     string
	image         string
	kubeconfig    string
	runtimeClass  string
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	config        *rest.Config
}

// sandboxGVR is the GroupVersionResource for the Sandbox CRD
var sandboxGVR = schema.GroupVersionResource{
	Group:    "agents.x-k8s.io",
	Version:  "v1alpha1",
	Resource: "sandboxes",
}

// AgentSandboxOption represents a configuration option for AgentSandbox
type AgentSandboxOption func(*AgentSandbox) error

// NewAgentSandbox creates a new AgentSandbox instance with the given name and options
func NewAgentSandbox(name string, opts ...AgentSandboxOption) (*AgentSandbox, error) {
	s := &AgentSandbox{
		name:      name,
		namespace: "computer", // default namespace
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	// Initialize Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", s.kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %v", err)
	}

	s.config = config
	s.clientset = clientset
	s.dynamicClient = dynamicClient

	return s, nil
}

// WithAgentSandboxKubeconfig sets the kubeconfig file path
func WithAgentSandboxKubeconfig(kubeconfig string) AgentSandboxOption {
	return func(s *AgentSandbox) error {
		s.kubeconfig = kubeconfig
		return nil
	}
}

// WithAgentSandboxNamespace sets the namespace
func WithAgentSandboxNamespace(namespace string) AgentSandboxOption {
	return func(s *AgentSandbox) error {
		s.namespace = namespace
		return nil
	}
}

// WithAgentSandboxImage sets the container image
func WithAgentSandboxImage(image string) AgentSandboxOption {
	return func(s *AgentSandbox) error {
		s.image = image
		return nil
	}
}

// WithRuntimeClass sets the RuntimeClass for enhanced isolation (e.g., "gvisor", "kata")
func WithRuntimeClass(runtimeClass string) AgentSandboxOption {
	return func(s *AgentSandbox) error {
		s.runtimeClass = runtimeClass
		return nil
	}
}

// Execute executes the command in the agent sandbox
func (s *AgentSandbox) Execute(ctx context.Context, command string, env []string, workDir string) (*ExecResult, error) {
	fullCommand := command

	// Ensure kubectl is in the PATH
	fullCommand = fmt.Sprintf("export PATH=/opt/bitnami/kubectl/bin:$PATH; %s", fullCommand)

	if workDir != "" {
		fullCommand = fmt.Sprintf("mkdir -p %q && cd %q && %s", workDir, workDir, fullCommand)
	}

	for _, envVar := range env {
		fullCommand = fmt.Sprintf("export %s; %s", envVar, fullCommand)
	}

	// Ensure sandbox exists and is ready
	if err := s.ensureSandbox(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure sandbox: %w", err)
	}

	// Wait for sandbox pod to be ready
	if err := s.waitForPodReady(ctx); err != nil {
		return nil, fmt.Errorf("failed waiting for sandbox pod: %w", err)
	}

	// Execute command in the pod
	var stdout, stderr bytes.Buffer
	err := s.executeInPod(ctx, fullCommand, &stdout, &stderr)

	result := &ExecResult{
		Command: command,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}
	if err != nil {
		result.Error = err.Error()
		result.ExitCode = 1
	}

	return result, nil
}

// Close cleans up the sandbox resources
func (s *AgentSandbox) Close(ctx context.Context) error {
	return s.deleteSandbox(ctx)
}

// ensureSandbox creates the Sandbox resource if it doesn't exist
func (s *AgentSandbox) ensureSandbox(ctx context.Context) error {
	// Check if sandbox already exists
	existing, err := s.getSandbox(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for existing sandbox: %w", err)
	}

	if existing != nil {
		// Sandbox exists, verify image matches
		existingImage, _, _ := unstructured.NestedString(existing.Object, "spec", "podTemplate", "spec", "containers", "0", "image")
		if existingImage != "" && existingImage != s.image {
			return fmt.Errorf(
				"existing sandbox '%s' uses image '%s', but new execution requested image '%s'. Please delete the sandbox first",
				s.name,
				existingImage,
				s.image,
			)
		}
		return nil
	}

	// Create new sandbox
	return s.createSandbox(ctx)
}

// createSandbox creates a new Sandbox resource
func (s *AgentSandbox) createSandbox(ctx context.Context) error {
	// First, create the kubeconfig ConfigMap
	configMapName := s.name + "-kubeconfig"
	if err := s.createKubeconfigMap(ctx, configMapName); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create kubeconfig configmap: %w", err)
		}
	}

	// Prepare pod template spec
	podSpec := map[string]interface{}{
		"serviceAccountName": "normal-user",
		"containers": []map[string]interface{}{
			{
				"name":  "main",
				"image": s.image,
				"command": []string{"sleep"},
				"args": []string{"infinity"},
				"env": []map[string]interface{}{
					{
						"name":  "KUBECONFIG",
						"value": "/etc/kube/config",
					},
					{
						"name":  "PATH",
						"value": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/bitnami/kubectl/bin",
					},
				},
				"volumeMounts": []map[string]interface{}{
					{
						"name":      "kubeconfig-volume",
						"mountPath": "/etc/kube",
						"readOnly":  true,
					},
				},
			},
		},
		"volumes": []map[string]interface{}{
			{
				"name": "kubeconfig-volume",
				"configMap": map[string]interface{}{
					"name": configMapName,
					"items": []map[string]interface{}{
						{
							"key":  "config",
							"path": "config",
						},
					},
				},
			},
		},
		"restartPolicy": "Never",
	}

	// Add runtimeClassName if specified
	if s.runtimeClass != "" {
		podSpec["runtimeClassName"] = s.runtimeClass
	}

	// Create Sandbox resource
	sandbox := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agents.x-k8s.io/v1alpha1",
			"kind":       "Sandbox",
			"metadata": map[string]interface{}{
				"name":      s.name,
				"namespace": s.namespace,
			},
			"spec": map[string]interface{}{
				"podTemplate": map[string]interface{}{
					"spec": podSpec,
				},
				"replicas": int64(1),
			},
		},
	}

	_, err := s.dynamicClient.Resource(sandboxGVR).Namespace(s.namespace).Create(ctx, sandbox, metav1.CreateOptions{})
	if err != nil {
		// Clean up configmap on failure
		if cleanupErr := s.deleteKubeconfigMap(ctx, configMapName); cleanupErr != nil {
			return fmt.Errorf("sandbox creation failed: %v; ALSO, configmap cleanup failed: %v", err, cleanupErr)
		}
		return fmt.Errorf("failed to create sandbox: %w", err)
	}

	return nil
}

// getSandbox retrieves the Sandbox resource
func (s *AgentSandbox) getSandbox(ctx context.Context) (*unstructured.Unstructured, error) {
	sandbox, err := s.dynamicClient.Resource(sandboxGVR).Namespace(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return sandbox, nil
}

// deleteSandbox removes the Sandbox resource
func (s *AgentSandbox) deleteSandbox(ctx context.Context) error {
	var errs []string

	// Delete Sandbox resource (with zero grace period)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: new(int64), // 0 seconds
	}
	err := s.dynamicClient.Resource(sandboxGVR).Namespace(s.namespace).Delete(ctx, s.name, deleteOptions)
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("failed to delete sandbox: %v", err))
	}

	// Delete ConfigMap
	configMapName := s.name + "-kubeconfig"
	if err := s.deleteKubeconfigMap(ctx, configMapName); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete kubeconfig configmap: %v", err))
	}

	// Wait for sandbox to be fully removed
	pollErr := wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, getErr := s.getSandbox(ctx)
		if errors.IsNotFound(getErr) {
			return true, nil // Sandbox is gone
		}
		if getErr != nil {
			return false, getErr
		}
		return false, nil // Still exists
	})
	if pollErr != nil {
		errs = append(errs, fmt.Sprintf("error waiting for sandbox deletion: %v", pollErr))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during sandbox deletion: %s", strings.Join(errs, "; "))
	}

	return nil
}

// createKubeconfigMap creates a ConfigMap with the kubeconfig for in-cluster usage
func (s *AgentSandbox) createKubeconfigMap(ctx context.Context, name string) error {
	kubeconfigYAML := fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    server: https://kubernetes.default.svc
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: default
contexts:
- context:
    cluster: default
    namespace: %s
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token`, s.namespace)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.namespace,
		},
		Data: map[string]string{
			"config": kubeconfigYAML,
		},
	}

	_, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Create(ctx, configMap, metav1.CreateOptions{})
	return err
}

// deleteKubeconfigMap deletes the ConfigMap
func (s *AgentSandbox) deleteKubeconfigMap(ctx context.Context, name string) error {
	err := s.clientset.CoreV1().ConfigMaps(s.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete kubeconfig configmap: %w", err)
	}
	return nil
}

// waitForPodReady waits for the sandbox pod to be ready
func (s *AgentSandbox) waitForPodReady(ctx context.Context) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		// Get the Sandbox resource to find the associated pod
		sandbox, err := s.getSandbox(ctx)
		if err != nil {
			return false, err
		}

		// Check sandbox conditions for Ready status
		conditions, found, err := unstructured.NestedSlice(sandbox.Object, "status", "conditions")
		if err != nil || !found {
			return false, nil // Not ready yet
		}

		for _, cond := range conditions {
			condition, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _, _ := unstructured.NestedString(condition, "type")
			condStatus, _, _ := unstructured.NestedString(condition, "status")
			if condType == "Ready" && condStatus == "True" {
				return true, nil
			}
		}

		return false, nil
	})
}

// executeInPod executes a command in the sandbox pod
func (s *AgentSandbox) executeInPod(ctx context.Context, command string, stdout, stderr io.Writer) error {
	// Find the pod created by the Sandbox controller
	// The pod name is typically the same as the sandbox name or has a predictable suffix
	podName := s.name

	// Default to os.Stdout/Stderr if not provided
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	req := s.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(s.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "main",
		Command:   []string{"/bin/sh", "-c", command},
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("error creating executor: %v", err)
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return fmt.Errorf("error executing command: %v", err)
	}

	return nil
}
