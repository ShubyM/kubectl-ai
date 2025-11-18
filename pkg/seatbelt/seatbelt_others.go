//go:build !darwin

package seatbelt

// WrapCommand is a no-op on non-macOS systems.
func WrapCommand(command string, args []string, profileName string) (string, []string, error) {
	return command, args, nil
}
