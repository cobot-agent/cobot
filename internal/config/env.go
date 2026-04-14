package config

import (
	"log/slog"
	"os"
	"regexp"
	"strings"
)

var envVarRe = regexp.MustCompile(`\$\{(\w+)\}`)

func ExpandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.Trim(match, "${}")
		val, ok := os.LookupEnv(varName)
		if !ok {
			slog.Warn("config: environment variable not set", "var", varName, "placeholder", match)
			return match // keep original ${VAR} instead of silently replacing with ""
		}
		return val
	})
}
