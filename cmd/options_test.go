package main

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
)

func TestResolveApprovalPolicy(t *testing.T) {
	tests := []struct {
		name    string
		opt     Options
		want    agent.ApprovalPolicy
		wantErr bool
	}{
		{
			name: "default to auto approve read",
		},
		{
			name: "skip permissions sets yolo",
			opt:  Options{SkipPermissions: true},
			want: agent.ApprovalPolicyYolo,
		},
		{
			name: "explicit paranoid preserved",
			opt:  Options{ApprovalPolicy: agent.ApprovalPolicyParanoid},
			want: agent.ApprovalPolicyParanoid,
		},
		{
			name:    "invalid policy",
			opt:     Options{ApprovalPolicy: agent.ApprovalPolicy("invalid")},
			wantErr: true,
		},
		{
			name: "skip permissions overridden by explicit",
			opt:  Options{SkipPermissions: true, ApprovalPolicy: agent.ApprovalPolicyParanoid},
			want: agent.ApprovalPolicyParanoid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := tt.opt
			err := opt.ResolveApprovalPolicy()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := tt.want
			if want == "" {
				want = agent.ApprovalPolicyAutoApproveRead
			}
			if opt.ApprovalPolicy != want {
				t.Fatalf("expected approval policy %q, got %q", want, opt.ApprovalPolicy)
			}
		})
	}
}
