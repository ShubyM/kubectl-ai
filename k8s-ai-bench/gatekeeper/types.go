package main

import "time"

// GitHubContent represents a GitHub API content response
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

// Constraint represents a Gatekeeper constraint with its examples
type Constraint struct {
	Name        string             `yaml:"name" json:"name"`
	Category    string             `yaml:"category" json:"category"`
	Description string             `yaml:"description" json:"description"`
	Template    string             `yaml:"template" json:"template"`
	Samples     []ConstraintSample `yaml:"samples" json:"samples"`
}

// ConstraintSample represents a sample constraint with allowed/disallowed examples
type ConstraintSample struct {
	Name               string   `yaml:"name" json:"name"`
	Constraint         string   `yaml:"constraint" json:"constraint"`
	AllowedExamples    []string `yaml:"allowed_examples" json:"allowed_examples"`
	DisallowedExamples []string `yaml:"disallowed_examples" json:"disallowed_examples"`
}

// ConstraintLibrary holds all fetched constraints
type ConstraintLibrary struct {
	FetchedAt   time.Time    `yaml:"fetched_at" json:"fetched_at"`
	Constraints []Constraint `yaml:"constraints" json:"constraints"`
}

// BenchmarkType defines the type of benchmark to generate
type BenchmarkType string

const (
	// PredictViolation - Given a constraint and resource, predict if it violates
	PredictViolation BenchmarkType = "predict-violation"
	// ExplainViolation - Explain why a resource violates a constraint
	ExplainViolation BenchmarkType = "explain-violation"
	// FixViolation - Fix a violating resource to make it compliant
	FixViolation BenchmarkType = "fix-violation"
	// AuditCluster - Audit cluster resources against a constraint
	AuditCluster BenchmarkType = "audit-cluster"
)

// BenchmarkTask represents a generated benchmark task
type BenchmarkTask struct {
	Name           string
	Constraint     Constraint
	Sample         ConstraintSample
	TestResource   string
	ExpectedResult string // "pass" or "fail"
	Type           BenchmarkType
}
