package skill

import "errors"

// ParseUploadOverwrite validates the shared skill-upload overwrite value.
func ParseUploadOverwrite(value string) (bool, error) {
	switch value {
	case "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, errors.New("--overwrite must be true or false")
	}
}
