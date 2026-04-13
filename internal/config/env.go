package config

import (
	"os"
	"regexp"
	"strings"
)

var envVarRe = regexp.MustCompile(`\$\{(\w+)\}`)

func ExpandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.Trim(match, "${}")
		return os.Getenv(varName)
	})
}
