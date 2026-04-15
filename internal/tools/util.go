package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// decodeArgs unmarshals JSON tool arguments into v.
func decodeArgs(args json.RawMessage, v any) error {
	if err := json.Unmarshal(args, v); err != nil {
		return fmt.Errorf("parse arguments: %w", err)
	}
	return nil
}

// validateName ensures a name does not contain path separators or parent references.
func validateName(name string) error {
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid name: must not contain path separators or parent directory references")
	}
	return nil
}
