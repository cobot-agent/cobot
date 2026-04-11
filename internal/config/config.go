package config

import (
	"os"
	"regexp"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
	"gopkg.in/yaml.v3"
)

var envVarRe = regexp.MustCompile(`\$\{(\w+)\}`)

func LoadFromFile(cfg *cobot.Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	expanded := expandEnvVars(string(data))
	return yaml.Unmarshal([]byte(expanded), cfg)
}

func expandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.Trim(match, "${}")
		return os.Getenv(varName)
	})
}
