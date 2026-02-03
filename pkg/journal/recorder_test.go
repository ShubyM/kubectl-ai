package journal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type stubRecorder struct {
	writes   []*Event
	closed   bool
	writeErr error
	closeErr error
}

func (s *stubRecorder) Write(ctx context.Context, event *Event) error {
	s.writes = append(s.writes, event)
	return s.writeErr
}

func (s *stubRecorder) Close() error {
	s.closed = true
	return s.closeErr
}

func TestMultiRecorder_WriteAndClose(t *testing.T) {
	r1 := &stubRecorder{}
	r2 := &stubRecorder{}

	recorder := NewMultiRecorder(r1, r2)

	event := &Event{Action: "test"}
	if err := recorder.Write(context.Background(), event); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	if len(r1.writes) != 1 || len(r2.writes) != 1 {
		t.Fatalf("writes were not fanned out: r1=%d r2=%d", len(r1.writes), len(r2.writes))
	}

	if err := recorder.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	if !r1.closed || !r2.closed {
		t.Fatalf("recorders were not closed: r1=%t r2=%t", r1.closed, r2.closed)
	}
}

func TestNewFileRecorderCreatesDirectories(t *testing.T) {
	root := t.TempDir()
	tracePath := filepath.Join(root, "nested", "trace.yaml")

	recorder, err := NewFileRecorder(tracePath)
	if err != nil {
		t.Fatalf("unexpected error creating recorder: %v", err)
	}
	defer recorder.Close()

	if _, err := os.Stat(filepath.Dir(tracePath)); err != nil {
		t.Fatalf("expected trace directory to exist: %v", err)
	}
}
