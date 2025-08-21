// Package mocks holds go:generate directives for gomock.
package mocks

// Generate gomock types for external interfaces we depend on.
// NOTE: run `go generate ./...` from repo root to (re)create mocks.
// Requires: go install go.uber.org/mock/mockgen@latest

// gollm interfaces
//   - Client, Chat
// tools interface
//   - Tool

//go:generate mockgen -destination=gollm_mock.go -package=mocks github.com/GoogleCloudPlatform/kubectl-ai/gollm Client,Chat
//go:generate mockgen -destination=tools_mock.go -package=mocks github.com/GoogleCloudPlatform/kubectl-ai/pkg/tools Tool
//go:generate mockgen -destination=agent_mock.go -package=mocks github.com/GoogleCloudPlatform/kubectl-ai/pkg/api ChatMessageStore
