//go:build !darwin

package seatbelt

// WrapCommand returns the command and arguments unchanged on non-Darwin systems.
func WrapCommand(command string, args []string, profileName string) (string, []string, error) {
	return command, args, nil
}
