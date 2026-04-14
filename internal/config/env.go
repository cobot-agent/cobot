package config

import (
	"fmt"
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
			fmt.Printf("config: warning: environment variable %s is not set, keeping placeholder %s\n", varName, match)
			return match // keep original ${VAR} instead of silently replacing with ""
		}
		return val
	})
}
