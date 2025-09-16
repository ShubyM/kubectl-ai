package agent

// ApprovalPolicy controls how the agent seeks permission before executing tool
// calls suggested by the model.
type ApprovalPolicy string

const (
	// ApprovalPolicyAutoApproveRead requests approval only when the tool
	// call is expected to modify cluster resources or when that is
	// unknown.
	ApprovalPolicyAutoApproveRead ApprovalPolicy = "auto-approve-read"

	// ApprovalPolicyParanoid always asks for approval before executing any
	// tool call, regardless of whether it is read-only.
	ApprovalPolicyParanoid ApprovalPolicy = "paranoid"

	// ApprovalPolicyYolo disables approval checks entirely.
	ApprovalPolicyYolo ApprovalPolicy = "yolo"
)

// IsValid reports whether the policy is one of the supported values.
func (p ApprovalPolicy) IsValid() bool {
	switch p {
	case ApprovalPolicyAutoApproveRead, ApprovalPolicyParanoid, ApprovalPolicyYolo:
		return true
	default:
		return false
	}
}
