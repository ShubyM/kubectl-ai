package exec

import (
	"context"
	"io"
)

// Result captures the outcome of a command.
// For streaming commands, Stdout/Stderr might be empty if they were consumed by the stream.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// StreamOptions defines the streams for a command.
type StreamOptions struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	TTY    bool
}

// Executor defines how a command is run.
type Executor interface {
	Run(ctx context.Context, command string, args []string, opts ...Option) (*Result, error)
}

type Config struct {
	Dir           string
	Env           []string
	StreamOptions StreamOptions
}

type Option func(*Config)

// WithDir sets the working directory for the command.
func WithDir(dir string) Option {
	return func(c *Config) {
		c.Dir = dir
	}
}

// WithEnv appends environment variables for the command execution.
func WithEnv(env []string) Option {
	return func(c *Config) {
		c.Env = append(c.Env, env...)
	}
}

// WithStreams sets the IO streams for the command.
func WithStreams(streams StreamOptions) Option {
	return func(c *Config) {
		c.StreamOptions = streams
	}
}
