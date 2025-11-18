package seatbelt

import "fmt"

const (
	// ProfilePermissiveOpen allows reading everything but restricts writing to specific locations.
	// This is a best-effort approximation of the gemini-cli profile.
	ProfilePermissiveOpen = "permissive-open"
	// ProfileStrict is a highly restrictive profile.
	ProfileStrict = "strict"
)

// defaultPermissiveProfile is a base template for the permissive profile.
// In a real implementation, we might want to template this with the current working directory.
const defaultPermissiveProfile = `(version 1)
(allow default)
(deny file-write*
    (subpath "/usr")
    (subpath "/bin")
    (subpath "/sbin")
    (subpath "/System")
    (subpath "/Library")
)
(allow file-write*
    (subpath "/usr/local/var")
    (subpath "/usr/local/Cellar")
    (subpath "/tmp")
    (subpath "/private/tmp")
    (subpath "/private/var/tmp")
)
`

const defaultStrictProfile = `(version 1)
(deny default)
(allow process-exec*)
(allow file-read* (subpath "/usr/lib"))
(allow file-read* (subpath "/usr/share"))
(allow file-read* (subpath "/System/Library"))
`

// GetProfile returns the content of the requested profile.
func GetProfile(name string) (string, error) {
	switch name {
	case ProfilePermissiveOpen:
		return defaultPermissiveProfile, nil
	case ProfileStrict:
		return defaultStrictProfile, nil
	default:
		return "", fmt.Errorf("unknown seatbelt profile: %s", name)
	}
}
