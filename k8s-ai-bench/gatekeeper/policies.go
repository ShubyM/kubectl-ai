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

package main

// PolicyDescriptions maps Gatekeeper policy names to human-readable descriptions.
// These descriptions are used in task prompts to explain the policy requirement
// in natural language, without exposing the underlying Rego policy syntax.
//
// When adding new policies, write descriptions that:
// - Clearly state what is required or prohibited
// - Reference specific Kubernetes fields when relevant
// - Are actionable and unambiguous
var PolicyDescriptions = map[string]string{
	// Pod Security Policies
	"privileged-containers": "Containers must NOT run in privileged mode. The securityContext.privileged field must be set to false or omitted.",

	"host-network-ports": "Pods must NOT use host networking or bind to host ports. The hostNetwork field must be false and hostPort must not be specified in container ports.",

	"host-namespaces": "Pods must NOT share the host's PID, IPC, or network namespaces. The fields hostPID, hostIPC, and hostNetwork must all be false or omitted.",

	"host-filesystem": "Pods must NOT mount sensitive host filesystem paths. The hostPath volume type should be restricted or disallowed entirely.",

	"read-only-root-filesystem": "Containers must use a read-only root filesystem. The securityContext.readOnlyRootFilesystem must be set to true.",

	"allow-privilege-escalation": "Containers must NOT allow privilege escalation. The securityContext.allowPrivilegeEscalation must be set to false.",

	"capabilities": "Container Linux capabilities must be restricted. Containers should drop all capabilities and only add specific required ones. Adding dangerous capabilities like NET_ADMIN, SYS_ADMIN, or ALL is prohibited.",

	"users": "Containers must run as a non-root user. The securityContext.runAsNonRoot must be true, and runAsUser should be set to a non-zero value.",

	"seccomp": "Pods must have a Seccomp profile configured. The securityContext.seccompProfile must be set to RuntimeDefault or a custom profile.",

	"seccompv2": "Pods must have a Seccomp profile configured using the native securityContext.seccompProfile field (not annotations). Allowed types are RuntimeDefault or Localhost.",

	"apparmor": "Pods must have an AppArmor profile configured. Containers should use the runtime/default profile or a custom profile, not be unconfined.",

	"selinux": "Pods must have appropriate SELinux options configured. The seLinuxOptions in securityContext must match allowed values for level, role, type, and user.",

	"proc-mount": "The /proc mount type must be Default, not Unmasked. The securityContext.procMount field must be set to Default or omitted.",

	"volumes": "Only specific volume types are allowed. Pods should not use hostPath or other dangerous volume types. Allowed types typically include configMap, emptyDir, projected, secret, downwardAPI, and persistentVolumeClaim.",

	"fsgroup": "Pods must configure the fsGroup security context appropriately. The fsGroup must be within allowed ranges.",

	"flexvolume-drivers": "FlexVolume drivers must be from an approved list. Only explicitly allowed FlexVolume drivers may be used.",

	"forbidden-sysctls": "Pods must not set forbidden sysctls. Certain kernel parameters are prohibited from being modified.",

	// General Policies
	"httpsonly": "Ingress resources must use HTTPS. The annotation kubernetes.io/ingress.allow-http must be set to 'false', and TLS configuration must be present.",

	"requiredlabels": "Resources must have all required labels with values matching specified patterns. Missing labels or labels with non-matching values are violations.",

	"requiredannotations": "Resources must have all required annotations. Missing annotations are violations.",

	"requiredprobes": "Containers must define readiness and/or liveness probes for health checking.",

	"containerlimits": "Containers must specify resource limits (CPU and memory). The resources.limits field must be defined.",

	"containerrequests": "Containers must specify resource requests (CPU and memory). The resources.requests field must be defined.",

	"containerresources": "Containers must specify both resource requests and limits for CPU and memory.",

	"containerresourceratios": "Container resource limits must be within a specified ratio of requests. The limit-to-request ratio cannot exceed the configured maximum.",

	"allowedrepos": "Container images must come from allowed repositories. Images must have a prefix matching one of the approved registries.",

	"allowedreposv2": "Container images must come from allowed repositories. This version supports exact matching and glob patterns for more precise control.",

	"disallowedrepos": "Container images must NOT come from disallowed repositories. Images from blocked registries are prohibited.",

	"disallowedtags": "Container images must NOT use disallowed tags. Tags like 'latest' are often prohibited to ensure reproducible deployments.",

	"imagedigests": "Container images must be specified by digest (@sha256:...), not by tag. This ensures immutable image references.",

	"replicalimits": "Deployments and ReplicaSets must have replica counts within allowed ranges. Both minimum and maximum replica limits are enforced.",

	"block-nodeport-services": "Services must NOT use the NodePort type. NodePort services expose ports on all cluster nodes and may be restricted.",

	"block-loadbalancer-services": "Services must NOT use the LoadBalancer type. LoadBalancer services provision external load balancers and may be restricted.",

	"block-wildcard-ingress": "Ingress resources must NOT use wildcard hosts. The host field must be a specific hostname, not a wildcard pattern.",

	"uniqueingresshost": "Ingress hosts must be unique across the cluster. No two Ingress resources can claim the same hostname.",

	"uniqueserviceselector": "Service selectors must be unique. No two Services should select the exact same set of pods.",

	"externalip": "Services must NOT specify external IPs, or must only use approved external IP addresses.",

	"storageclass": "PersistentVolumeClaims must use storage classes from an allowed list. Only approved storage classes may be requested.",

	"poddisruptionbudget": "PodDisruptionBudgets must be configured appropriately. minAvailable and maxUnavailable must meet requirements.",

	"horizontalpodautoscaler": "HorizontalPodAutoscalers must have appropriate minimum and maximum replica bounds configured.",

	"disallowanonymous": "RBAC bindings must NOT grant permissions to anonymous users or the system:unauthenticated group. This prevents unauthenticated access.",

	"block-endpoint-edit-default-role": "The system:aggregate-to-edit ClusterRole must NOT include permissions to modify Endpoints or EndpointSlices.",

	"verifydeprecatedapi": "Resources must NOT use deprecated API versions. API versions that have been removed in newer Kubernetes versions are prohibited.",

	"noupdateserviceaccount": "Pod service accounts must NOT be changed after creation. The serviceAccountName field is immutable.",

	"disallowinteractive": "Pods must NOT enable interactive TTY or stdin. The tty and stdin fields must be false or omitted.",

	"automount-serviceaccount-token": "Pods should explicitly configure service account token automounting. The automountServiceAccountToken field should be explicitly set.",

	"ephemeralstoragelimit": "Containers must specify ephemeral storage limits. The resources.limits.ephemeral-storage field must be defined.",
}
