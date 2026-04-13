package cobot

import (
	"encoding/json"
	"testing"
)

// Ensure that Tools migration to DefaultTools works when loading JSON config
func TestConfigMigration_ToolsToDefaultTools_JSON(t *testing.T) {
	data := []byte(`{"tools": {"builtin": ["cmd1","cmd2"]}}`)
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}
	// Before migration, DefaultTools should be empty
	if len(cfg.DefaultTools.Builtin) != 0 || len(cfg.DefaultTools.MCPServers) != 0 {
		t.Fatalf("expected default_tools to be empty before migrate, got: %#v", cfg.DefaultTools)
	}
	cfg.migrateTools()
	if len(cfg.DefaultTools.Builtin) != 2 || cfg.DefaultTools.Builtin[0] != "cmd1" {
		t.Fatalf("migration failed, got: %#v", cfg.DefaultTools)
	}
}

// Ensure that Temperature serializes and deserializes correctly and that JSON includes default_tools when populated
func TestConfig_Temperature_JSONSerialization(t *testing.T) {
	cfg := Config{Temperature: 12.5, DefaultTools: ToolsConfig{Builtin: []string{"cmd"}}}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	// Decode to map for easy assertion
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal back err: %v", err)
	}
	if v, ok := m["temperature"]; !ok {
		t.Fatalf("missing temperature in json: %v", m)
	} else if v.(float64) != 12.5 {
		t.Fatalf("temperature mismatch: %v", v)
	}
	if _, ok := m["default_tools"]; !ok {
		t.Fatalf("missing default_tools in json: %v", m)
	}
}
