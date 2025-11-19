//go:build darwin

package seatbelt

import (
	"fmt"
	"os"
)

// WrapCommand wraps the given command and arguments with sandbox-exec using the specified profile.
func WrapCommand(command string, args []string, profileName string) (string, []string, error) {
	if profileName == "" {
		return command, args, nil
	}

	profileContent, err := GetProfile(profileName)
	if err != nil {
		// If it's not a built-in profile, check if it's a file path
		if _, statErr := os.Stat(profileName); statErr == nil {
			content, readErr := os.ReadFile(profileName)
			if readErr != nil {
				return "", nil, fmt.Errorf("failed to read custom profile file: %w", readErr)
			}
			profileContent = string(content)
		} else {
			return "", nil, err
		}
	}

	// We need to pass the profile content to sandbox-exec.
	// sandbox-exec -p PROFILE_CONTENT command args...
	
	newArgs := []string{"-p", profileContent, command}
	newArgs = append(newArgs, args...)

	return "sandbox-exec", newArgs, nil
}
