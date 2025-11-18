package exec

import (
	"context"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/seatbelt"
)

// SeatbeltExecutor wraps another executor and applies seatbelt profiles.
type SeatbeltExecutor struct {
	delegate Executor
	profile  string
}

// NewSeatbeltExecutor creates a new SeatbeltExecutor.
func NewSeatbeltExecutor(delegate Executor, profile string) *SeatbeltExecutor {
	return &SeatbeltExecutor{
		delegate: delegate,
		profile:  profile,
	}
}

// Run executes the command using the delegate executor, wrapping it with the seatbelt profile.
func (e *SeatbeltExecutor) Run(ctx context.Context, command string, args []string, opts ...Option) (*Result, error) {
	wrappedCmd, wrappedArgs, err := seatbelt.WrapCommand(command, args, e.profile)
	if err != nil {
		return nil, err
	}
	return e.delegate.Run(ctx, wrappedCmd, wrappedArgs, opts...)
}
