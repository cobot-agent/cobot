# Config, Skills, MCP & Workspace Architecture Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure cobot's config, skills, MCP, and workspace management into a two-tier mutability system with registry+enable pattern, full sandbox, multi-agent per workspace, and configurable base paths.

**Architecture:** Two-tier directory layout — `<config_dir>` (agent-immutable) holds global config, MCP registry, skills registry, workspace definitions; `<data_dir>` (agent-mutable) holds per-workspace runtime config, agents, skills, memory, sessions. Workspaces enable MCP/skills from global registries. Sandboxing enforced at tool execution layer.

**Tech Stack:** Go 1.26, gopkg.in/yaml.v3, github.com/modelcontextprotocol/go-sdk, github.com/spf13/cobra

**Spec:** `docs/specs/2026-04-12-config-skills-mcp-workspace-design.md`

---

## Phase 1: Foundation — Path Resolution & New Config Types

### Task 1: Path Resolution Layer

**Files:**
- Modify: `internal/xdg/xdg.go`
- Create: `internal/xdg/xdg_test.go`

This task replaces the hardcoded XDG paths with a resolution system that supports `COBOT_CONFIG_PATH`, `COBOT_DATA_PATH` env vars.

- [ ] **Step 1: Write failing tests for path resolution**

```go
// internal/xdg/xdg_test.go
package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDir_Default(t *testing.T) {
	os.Unsetenv("COBOT_CONFIG_PATH")
	os.Unsetenv("XDG_CONFIG_HOME")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "cobot")
	if got := ConfigDir(); got != expected {
		t.Errorf("ConfigDir() = %q, want %q", got, expected)
	}
}

func TestConfigDir_EnvOverride(t *testing.T) {
	os.Setenv("COBOT_CONFIG_PATH", "/custom/config")
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	if got := ConfigDir(); got != "/custom/config" {
		t.Errorf("ConfigDir() = %q, want /custom/config", got)
	}
}

func TestDataDir_Default(t *testing.T) {
	os.Unsetenv("COBOT_DATA_PATH")
	os.Unsetenv("XDG_DATA_HOME")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", "cobot")
	if got := DataDir(); got != expected {
		t.Errorf("DataDir() = %q, want %q", got, expected)
	}
}

func TestDataDir_EnvOverride(t *testing.T) {
	os.Setenv("COBOT_DATA_PATH", "/custom/data")
	defer os.Unsetenv("COBOT_DATA_PATH")
	if got := DataDir(); got != "/custom/data" {
		t.Errorf("DataDir() = %q, want /custom/data", got)
	}
}

func TestMCPRegistryDir(t *testing.T) {
	os.Setenv("COBOT_CONFIG_PATH", "/tmp/test-cobot-config")
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	expected := "/tmp/test-cobot-config/mcp"
	if got := MCPRegistryDir(); got != expected {
		t.Errorf("MCPRegistryDir() = %q, want %q", got, expected)
	}
}

func TestSkillsRegistryDir(t *testing.T) {
	os.Setenv("COBOT_CONFIG_PATH", "/tmp/test-cobot-config")
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	expected := "/tmp/test-cobot-config/skills"
	if got := SkillsRegistryDir(); got != expected {
		t.Errorf("SkillsRegistryDir() = %q, want %q", got, expected)
	}
}

func TestWorkspaceDefinitionsDir(t *testing.T) {
	os.Setenv("COBOT_CONFIG_PATH", "/tmp/test-cobot-config")
	defer os.Unsetenv("COBOT_CONFIG_PATH")
	expected := "/tmp/test-cobot-config/workspaces"
	if got := WorkspaceDefinitionsDir(); got != expected {
		t.Errorf("WorkspaceDefinitionsDir() = %q, want %q", got, expected)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/xdg/ -v`
Expected: FAIL — `ConfigDir`, `DataDir` etc. not yet defined with new signatures

- [ ] **Step 3: Rewrite xdg.go with new path resolution**

```go
// internal/xdg/xdg.go
package xdg

import (
	"os"
	"path/filepath"
)

func ConfigDir() string {
	if v := os.Getenv("COBOT_CONFIG_PATH"); v != "" {
		return v
	}
	return filepath.Join(configHome(), "cobot")
}

func DataDir() string {
	if v := os.Getenv("COBOT_DATA_PATH"); v != "" {
		return v
	}
	return filepath.Join(dataHome(), "cobot")
}

func GlobalConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func MCPRegistryDir() string {
	return filepath.Join(ConfigDir(), "mcp")
}

func SkillsRegistryDir() string {
	return filepath.Join(ConfigDir(), "skills")
}

func WorkspaceDefinitionsDir() string {
	return filepath.Join(ConfigDir(), "workspaces")
}

func UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func configHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func dataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/xdg/ -v`
Expected: PASS

- [ ] **Step 5: Fix all callers of old xdg functions**

Old API: `xdg.CobotConfigDir()`, `xdg.CobotDataDir()` → New API: `xdg.ConfigDir()`, `xdg.DataDir()`

Run: `grep -rn 'xdg\.CobotConfigDir\|xdg\.CobotDataDir' --include='*.go' .`

Replace all occurrences:
- `xdg.CobotConfigDir()` → `xdg.ConfigDir()`
- `xdg.CobotDataDir()` → `xdg.DataDir()`

Files that reference these (update each):
- `internal/workspace/workspace.go` — `NewWorkspace()` function
- `internal/workspace/manager.go` — `NewManager()`, `managerConfigPath()`
- `internal/workspace/discovery.go` — `Discover()`
- `cmd/cobot/root.go` — `loadConfig()`

- [ ] **Step 6: Build and run all tests**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: replace XDG helpers with configurable path resolution"
```

---

### Task 2: New Config Types — SandboxConfig, AgentConfig, WorkspaceConfig

**Files:**
- Modify: `pkg/options.go`
- Create: `pkg/options_test.go`

Replace the old `Config` and related types with the new design. Remove `ToolsConfig`, `MCPServerConfig` from `pkg/options.go` — they move to dedicated packages.

- [ ] **Step 1: Write failing tests for new config types**

```go
// pkg/options_test.go
package cobot

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model != "openai:gpt-4o" {
		t.Errorf("Default Model = %q, want openai:gpt-4o", cfg.Model)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("Default MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if cfg.Memory.Enabled != true {
		t.Error("Default Memory.Enabled should be true")
	}
}

func TestSandboxConfig_IsAllowedPath(t *testing.T) {
	s := SandboxConfig{
		Root:        "/project",
		AllowPaths:  []string{"/tmp"},
		ReadonlyPaths: []string{"/etc/config"},
	}

	tests := []struct {
		path      string
		write     bool
		allowed   bool
	}{
		{"/project/src/main.go", true, true},
		{"/project", true, true},
		{"/tmp/output.txt", true, true},
		{"/etc/config/app.yaml", false, true},
		{"/etc/config/app.yaml", true, false},
		{"/usr/bin/evil", true, false},
		{"/project/../etc/passwd", true, false},
	}

	for _, tt := range tests {
		got := s.IsAllowed(tt.path, tt.write)
		if got != tt.allowed {
			t.Errorf("IsAllowed(%q, write=%v) = %v, want %v", tt.path, tt.write, got, tt.allowed)
		}
	}
}

func TestSandboxConfig_IsBlockedCommand(t *testing.T) {
	s := SandboxConfig{
		BlockedCommands: []string{"rm -rf /", "mkfs"},
	}

	if !s.IsBlockedCommand("rm -rf /") {
		t.Error("should block 'rm -rf /'")
	}
	if s.IsBlockedCommand("ls -la") {
		t.Error("should not block 'ls -la'")
	}
	if !s.IsBlockedCommand("sudo mkfs /dev/sda1") {
		t.Error("should block command containing 'mkfs'")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/ -run "TestDefaultConfig|TestSandboxConfig" -v`
Expected: FAIL — `SandboxConfig` type not defined

- [ ] **Step 3: Rewrite pkg/options.go with new types**

```go
// pkg/options.go
package cobot

import (
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ConfigPath  string             `yaml:"config_path,omitempty"`
	DataPath    string             `yaml:"data_path,omitempty"`
	Workspace   string             `yaml:"workspace,omitempty"`
	Model       string             `yaml:"model"`
	Temperature float64            `yaml:"temperature,omitempty"`
	MaxTurns    int                `yaml:"max_turns"`
	SystemPrompt string            `yaml:"system_prompt,omitempty"`
	Verbose     bool               `yaml:"verbose,omitempty"`
	APIKeys     map[string]string  `yaml:"api_keys,omitempty"`
	Providers   map[string]ProviderConfig `yaml:"providers,omitempty"`
	Memory      MemoryConfig       `yaml:"memory,omitempty"`
}

type MemoryConfig struct {
	Enabled             bool          `yaml:"enabled"`
	IntelligentCuration bool          `yaml:"intelligent_curation"`
	CurationInterval    time.Duration `yaml:"curation_interval"`
	BadgerPath          string        `yaml:"badger_path,omitempty"`
	BlevePath           string        `yaml:"bleve_path,omitempty"`
}

type ProviderConfig struct {
	BaseURL string            `yaml:"base_url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type SandboxConfig struct {
	Root            string   `yaml:"root"`
	AllowPaths      []string `yaml:"allow_paths,omitempty"`
	ReadonlyPaths   []string `yaml:"readonly_paths,omitempty"`
	AllowNetwork    bool     `yaml:"allow_network"`
	BlockedCommands []string `yaml:"blocked_commands,omitempty"`
}

func (s *SandboxConfig) IsAllowed(path string, write bool) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, p := range s.ReadonlyPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return !write
		}
	}

	for _, p := range s.AllowPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return true
		}
	}

	if s.Root != "" {
		absRoot, _ := filepath.Abs(s.Root)
		if isSubpath(absPath, absRoot) {
			return true
		}
	}

	return false
}

func (s *SandboxConfig) IsBlockedCommand(cmd string) bool {
	for _, blocked := range s.BlockedCommands {
		if strings.Contains(cmd, blocked) {
			return true
		}
	}
	return false
}

func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "."
}

func DefaultConfig() *Config {
	return &Config{
		MaxTurns: 50,
		Model:    "openai:gpt-4o",
		APIKeys:  make(map[string]string),
		Memory: MemoryConfig{
			Enabled:             true,
			IntelligentCuration: true,
			CurationInterval:    30 * time.Second,
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/ -v`
Expected: PASS

- [ ] **Step 5: Fix compilation errors from removed types**

Old types `ToolsConfig`, `MCPServerConfig`, `migrateTools()` are removed. Find and update all references:

Run: `grep -rn 'ToolsConfig\|MCPServerConfig\|migrateTools\|DefaultTools\|\.Tools\.' --include='*.go' .`

Key files to update:
- `internal/config/config.go` — remove references to old tool config
- `cmd/cobot/helpers.go` — remove builtin tool registration from config
- `cmd/cobot/tools.go` — update tool listing to use new registry approach
- Any other files referencing the removed types

For now, remove dead code. Tool registration will be rewired in later tasks.

- [ ] **Step 6: Build and test**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: replace Config types with new SandboxConfig, remove old ToolsConfig"
```

---

### Task 3: New Workspace Definition & Manager

**Files:**
- Rewrite: `internal/workspace/workspace.go`
- Rewrite: `internal/workspace/manager.go`
- Rewrite: `internal/workspace/discovery.go`
- Create: `internal/workspace/workspace_test.go`
- Create: `internal/workspace/manager_test.go`

This task replaces the workspace system: definitions live as flat YAML files in `<config_dir>/workspaces/<name>.yaml`, data lives under `<data_dir>/<name>/`.

- [ ] **Step 1: Write failing tests for WorkspaceDefinition**

```go
// internal/workspace/workspace_test.go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceDefinition_DefaultPath(t *testing.T) {
	def := &WorkspaceDefinition{
		Name: "my-project",
		Type: WorkspaceTypeProject,
		Root: "/path/to/project",
	}
	// When Path is empty, it should default to <data_dir>/<name>
	got := def.ResolvePath("/data")
	want := "/data/my-project"
	if got != want {
		t.Errorf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestWorkspaceDefinition_CustomPath(t *testing.T) {
	def := &WorkspaceDefinition{
		Name: "large-project",
		Type: WorkspaceTypeCustom,
		Path: "/Volumes/storage/large-project",
	}
	got := def.ResolvePath("/data")
	want := "/Volumes/storage/large-project"
	if got != want {
		t.Errorf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestSaveAndLoadDefinition(t *testing.T) {
	dir := t.TempDir()
	def := &WorkspaceDefinition{
		Name: "test-ws",
		Type: WorkspaceTypeCustom,
		Path: "/custom/path",
		Root: "/custom/root",
	}

	path := filepath.Join(dir, "test-ws.yaml")
	if err := saveDefinition(def, path); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != def.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, def.Name)
	}
	if loaded.Path != def.Path {
		t.Errorf("Path = %q, want %q", loaded.Path, def.Path)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/workspace/ -v`
Expected: FAIL

- [ ] **Step 3: Rewrite workspace.go**

```go
// internal/workspace/workspace.go
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type WorkspaceType string

const (
	WorkspaceTypeDefault WorkspaceType = "default"
	WorkspaceTypeProject WorkspaceType = "project"
	WorkspaceTypeCustom  WorkspaceType = "custom"
)

// WorkspaceDefinition lives in <config_dir>/workspaces/<name>.yaml
// Agent-immutable. Maps workspace name to its data directory.
type WorkspaceDefinition struct {
	Name string        `yaml:"name"`
	Type WorkspaceType `yaml:"type"`
	Path string        `yaml:"path,omitempty"`
	Root string        `yaml:"root,omitempty"`
}

// ResolvePath returns the data directory for this workspace.
// If Path is set, uses it; otherwise defaults to <dataDir>/<name>.
func (d *WorkspaceDefinition) ResolvePath(dataDir string) string {
	if d.Path != "" {
		return d.Path
	}
	return filepath.Join(dataDir, d.Name)
}

// WorkspaceConfig lives in <data_dir>/<ws>/workspace.yaml
// Agent-mutable. Runtime configuration for the workspace.
type WorkspaceConfig struct {
	ID           string            `yaml:"id"`
	Name         string            `yaml:"name"`
	Type         WorkspaceType     `yaml:"type"`
	Root         string            `yaml:"root,omitempty"`
	CreatedAt    time.Time         `yaml:"created_at"`
	UpdatedAt    time.Time         `yaml:"updated_at"`
	EnabledMCP   []string          `yaml:"enabled_mcp,omitempty"`
	EnabledSkills []string         `yaml:"enabled_skills,omitempty"`
	Sandbox      SandboxConfig     `yaml:"sandbox,omitempty"`
	Agents       map[string]string `yaml:"agents,omitempty"`
	DefaultAgent string            `yaml:"default_agent,omitempty"`
	Summarization *SummarizationConfig `yaml:"summarization,omitempty"`
}

// SandboxConfig is workspace-level sandbox settings.
// Duplicated from pkg to avoid circular imports.
type SandboxConfig struct {
	Root            string   `yaml:"root,omitempty"`
	AllowPaths      []string `yaml:"allow_paths,omitempty"`
	ReadonlyPaths   []string `yaml:"readonly_paths,omitempty"`
	AllowNetwork    bool     `yaml:"allow_network"`
	BlockedCommands []string `yaml:"blocked_commands,omitempty"`
}

type SummarizationConfig struct {
	Enabled bool     `yaml:"enabled"`
	Include []string `yaml:"include,omitempty"`
}

// Workspace is the runtime handle combining definition + config + resolved paths.
type Workspace struct {
	Definition *WorkspaceDefinition
	Config     *WorkspaceConfig
	DataDir    string
}

func (w *Workspace) IsDefault() bool {
	return w.Definition.Type == WorkspaceTypeDefault
}

func (w *Workspace) IsProject() bool {
	return w.Definition.Type == WorkspaceTypeProject
}

func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.DataDir, "memory")
}

func (w *Workspace) SessionsDir() string {
	return filepath.Join(w.DataDir, "sessions")
}

func (w *Workspace) SkillsDir() string {
	return filepath.Join(w.DataDir, "skills")
}

func (w *Workspace) SchedulerDir() string {
	return filepath.Join(w.DataDir, "scheduler")
}

func (w *Workspace) AgentsDir() string {
	return filepath.Join(w.DataDir, "agents")
}

func (w *Workspace) GetSoulPath() string {
	return filepath.Join(w.DataDir, "SOUL.md")
}

func (w *Workspace) GetUserPath() string {
	return filepath.Join(w.DataDir, "USER.md")
}

func (w *Workspace) GetMemoryMdPath() string {
	return filepath.Join(w.DataDir, "MEMORY.md")
}

func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.DataDir, "workspace.yaml")
}

func (w *Workspace) AgentsMdPath() string {
	if w.Definition.Root == "" {
		return ""
	}
	return filepath.Join(w.Definition.Root, ".cobot", "AGENTS.md")
}

func (w *Workspace) EnsureDirs() error {
	dirs := []string{
		w.DataDir,
		w.MemoryDir(),
		w.SessionsDir(),
		w.SkillsDir(),
		w.AgentsDir(),
		w.SchedulerDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (w *Workspace) SaveConfig() error {
	w.Config.UpdatedAt = time.Now()
	data, err := yaml.Marshal(w.Config)
	if err != nil {
		return fmt.Errorf("marshal workspace config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(w.ConfigPath()), 0755); err != nil {
		return err
	}
	return os.WriteFile(w.ConfigPath(), data, 0644)
}

func (w *Workspace) ValidatePath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	if isSubpath(absPath, w.DataDir) {
		return nil
	}
	if w.Definition.Root != "" {
		absRoot, _ := filepath.Abs(w.Definition.Root)
		if isSubpath(absPath, absRoot) {
			return nil
		}
	}
	for _, p := range w.Config.Sandbox.AllowPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return nil
		}
	}
	for _, p := range w.Config.Sandbox.ReadonlyPaths {
		absP, _ := filepath.Abs(p)
		if isSubpath(absPath, absP) {
			return nil
		}
	}

	return fmt.Errorf("path %s is outside workspace boundaries", path)
}

func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func saveDefinition(d *WorkspaceDefinition, path string) error {
	data, err := yaml.Marshal(d)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadDefinition(path string) (*WorkspaceDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d WorkspaceDefinition
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func loadWorkspaceConfig(path string) (*WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg WorkspaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func newWorkspaceConfig(name string, wsType WorkspaceType, root string) *WorkspaceConfig {
	return &WorkspaceConfig{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      wsType,
		Root:      root,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Agents:    map[string]string{"main": "agents/main.yaml"},
		DefaultAgent: "main",
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/workspace/ -v`
Expected: PASS

- [ ] **Step 5: Rewrite manager.go — remove current workspace tracking**

```go
// internal/workspace/manager.go
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cobot-agent/cobot/internal/xdg"
)

type Manager struct {
	definitionsDir string
	dataDir        string
}

func NewManager() (*Manager, error) {
	m := &Manager{
		definitionsDir: xdg.WorkspaceDefinitionsDir(),
		dataDir:        xdg.DataDir(),
	}
	if err := os.MkdirAll(m.definitionsDir, 0755); err != nil {
		return nil, fmt.Errorf("create definitions dir: %w", err)
	}
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := m.ensureDefault(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) ensureDefault() error {
	defPath := filepath.Join(m.definitionsDir, "default.yaml")
	if _, err := os.Stat(defPath); os.IsNotExist(err) {
		def := &WorkspaceDefinition{
			Name: "default",
			Type: WorkspaceTypeDefault,
		}
		if err := saveDefinition(def, defPath); err != nil {
			return err
		}
	}
	dataPath := filepath.Join(m.dataDir, "default")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return err
	}
	cfgPath := filepath.Join(dataPath, "workspace.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := newWorkspaceConfig("default", WorkspaceTypeDefault, "")
		data, _ := yaml.Marshal(cfg)
		if err := os.WriteFile(cfgPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Resolve(name string) (*Workspace, error) {
	defPath := filepath.Join(m.definitionsDir, name+".yaml")
	def, err := loadDefinition(defPath)
	if err != nil {
		return nil, fmt.Errorf("workspace %q not found: %w", name, err)
	}
	dataDir := def.ResolvePath(m.dataDir)
	cfgPath := filepath.Join(dataDir, "workspace.yaml")
	cfg, err := loadWorkspaceConfig(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = newWorkspaceConfig(def.Name, def.Type, def.Root)
		} else {
			return nil, err
		}
	}
	return &Workspace{
		Definition: def,
		Config:     cfg,
		DataDir:    dataDir,
	}, nil
}

func (m *Manager) Create(name string, wsType WorkspaceType, root string, customPath string) (*Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace name cannot be empty")
	}
	if name == "default" && wsType != WorkspaceTypeDefault {
		return nil, fmt.Errorf("name 'default' is reserved")
	}
	defPath := filepath.Join(m.definitionsDir, name+".yaml")
	if _, err := os.Stat(defPath); err == nil {
		return nil, fmt.Errorf("workspace %q already exists", name)
	}

	def := &WorkspaceDefinition{
		Name: name,
		Type: wsType,
		Path: customPath,
		Root: root,
	}
	if err := saveDefinition(def, defPath); err != nil {
		return nil, err
	}

	ws, err := m.Resolve(name)
	if err != nil {
		return nil, err
	}
	if err := ws.EnsureDirs(); err != nil {
		return nil, err
	}
	if err := ws.SaveConfig(); err != nil {
		return nil, err
	}
	return ws, nil
}

func (m *Manager) List() ([]*WorkspaceDefinition, error) {
	entries, err := os.ReadDir(m.definitionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var defs []*WorkspaceDefinition
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		def, err := loadDefinition(filepath.Join(m.definitionsDir, entry.Name()))
		if err != nil {
			continue
		}
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].Type == WorkspaceTypeDefault {
			return true
		}
		if defs[j].Type == WorkspaceTypeDefault {
			return false
		}
		return defs[i].Name < defs[j].Name
	})
	return defs, nil
}

func (m *Manager) Delete(name string) error {
	if name == "default" {
		return fmt.Errorf("cannot delete default workspace")
	}
	defPath := filepath.Join(m.definitionsDir, name+".yaml")
	def, err := loadDefinition(defPath)
	if err != nil {
		return fmt.Errorf("workspace %q not found", name)
	}
	if err := os.Remove(defPath); err != nil {
		return err
	}
	dataDir := def.ResolvePath(m.dataDir)
	return os.RemoveAll(dataDir)
}

func (m *Manager) Discover(startDir string) (*Workspace, error) {
	dir := startDir
	for {
		cobotDir := filepath.Join(dir, ".cobot")
		info, err := os.Stat(cobotDir)
		if err == nil && info.IsDir() {
			projectName := filepath.Base(dir)
			defPath := filepath.Join(m.definitionsDir, projectName+".yaml")
			if _, err := os.Stat(defPath); err == nil {
				return m.Resolve(projectName)
			}
			return m.Create(projectName, WorkspaceTypeProject, dir, "")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("no .cobot directory found from %s", startDir)
}

func (m *Manager) ResolveByNameOrDiscover(name string, startDir string) (*Workspace, error) {
	if name != "" {
		return m.Resolve(name)
	}
	if ws, err := m.Discover(startDir); err == nil {
		return ws, nil
	}
	return m.Resolve("default")
}
```

Note: `yaml` import needs `"gopkg.in/yaml.v3"` — add it to the import block.

- [ ] **Step 6: Rewrite discovery.go — simplify**

```go
// internal/workspace/discovery.go
package workspace

// Discovery is now handled directly by Manager.Discover() and Manager.ResolveByNameOrDiscover().
// This file is kept for backward compatibility wrappers.

func Discover(startDir string) (*Workspace, error) {
	m, err := NewManager()
	if err != nil {
		return nil, err
	}
	return m.Discover(startDir)
}

func DiscoverOrDefault(startDir string) (*Workspace, error) {
	m, err := NewManager()
	if err != nil {
		return nil, err
	}
	return m.ResolveByNameOrDiscover("", startDir)
}
```

- [ ] **Step 7: Build and fix compilation errors**

Run: `go build ./...`

Fix all callers that reference old `Workspace` struct fields:
- `ConfigDir` → `DataDir` (config now lives in data dir)
- `ConfigPath` → `ConfigPath()` method
- `Root` → `Definition.Root`
- `Name` → `Definition.Name` or `Config.Name`
- `ID` → `Config.ID`
- `Type` → `Definition.Type`

Key files to update:
- `cmd/cobot/root.go`
- `cmd/cobot/helpers.go`
- `cmd/cobot/workspace.go`
- `cmd/cobot/persona_cmd.go`
- `internal/agent/agent.go`
- `internal/persona/persona.go`

For each file, replace old field access with new struct layout.

- [ ] **Step 8: Run all tests**

Run: `go test ./...`
Expected: All pass

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "refactor: rewrite workspace system with definition/config split and no current tracking"
```

---

## Phase 2: MCP Registry

### Task 4: MCP Registry — Types & Loading

**Files:**
- Create: `internal/mcp/registry.go`
- Create: `internal/mcp/registry_test.go`

- [ ] **Step 1: Write failing tests for MCP registry**

```go
// internal/mcp/registry_test.go
package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()

	yaml1 := `name: github
description: GitHub API
transport: command
command: npx
args:
  - "@modelcontextprotocol/server-github"
env:
  GITHUB_TOKEN: test-token
`
	if err := os.WriteFile(filepath.Join(dir, "github.yaml"), []byte(yaml1), 0644); err != nil {
		t.Fatal(err)
	}

	yaml2 := `name: remote-api
description: Remote API
transport: http
url: http://localhost:8080
headers:
  Authorization: Bearer token123
`
	if err := os.WriteFile(filepath.Join(dir, "remote-api.yaml"), []byte(yaml2), 0644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(reg))
	}
	if reg["github"].Transport != "command" {
		t.Error("github transport should be command")
	}
	if reg["remote-api"].Transport != "http" {
		t.Error("remote-api transport should be http")
	}
	if reg["github"].Command != "npx" {
		t.Error("github command should be npx")
	}
}

func TestLoadRegistry_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore"), 0644)
	os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte("name: test\ntransport: command\ncommand: echo\n"), 0644)

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(reg))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mcp/ -v`
Expected: FAIL

- [ ] **Step 3: Implement MCP registry**

```go
// internal/mcp/registry.go
package mcp

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RegistryEntry struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Transport   string            `yaml:"transport"`
	Command     string            `yaml:"command,omitempty"`
	Args        []string          `yaml:"args,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	URL         string            `yaml:"url,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
}

func LoadRegistry(dir string) (map[string]*RegistryEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*RegistryEntry), nil
		}
		return nil, fmt.Errorf("read MCP registry dir: %w", err)
	}

	registry := make(map[string]*RegistryEntry)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var re RegistryEntry
		if err := yaml.Unmarshal(data, &re); err != nil {
			continue
		}
		if re.Name == "" {
			re.Name = strings.TrimSuffix(entry.Name(), ".yaml")
		}
		registry[re.Name] = &re
	}
	return registry, nil
}
```

Add `"strings"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mcp/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add MCP registry with directory-based YAML loading"
```

---

### Task 5: MCP Manager Integration with Registry

**Files:**
- Modify: `internal/mcp/manager.go`
- Create: `internal/mcp/manager_test.go` (extend)

Wire the MCP manager to connect servers from registry entries filtered by workspace's enabled list.

- [ ] **Step 1: Add ConnectFromRegistry method to MCPManager**

```go
// Add to internal/mcp/manager.go

func (m *MCPManager) ConnectFromRegistry(ctx context.Context, name string, entry *RegistryEntry) error {
	cfg := ServerConfig{
		Command: entry.Command,
		Args:    entry.Args,
	}

	if len(entry.Env) > 0 {
		for _, v := range entry.Env {
			cfg.Env = append(cfg.Env, v)
		}
	}

	return m.Connect(ctx, name, cfg)
}

func (m *MCPManager) ConnectEnabled(ctx context.Context, registry map[string]*RegistryEntry, enabled []string) error {
	for _, name := range enabled {
		entry, ok := registry[name]
		if !ok {
			return fmt.Errorf("MCP server %q not found in registry", name)
		}
		if err := m.ConnectFromRegistry(ctx, name, entry); err != nil {
			return fmt.Errorf("connect MCP server %q: %w", name, err)
		}
	}
	return nil
}
```

- [ ] **Step 2: Add env var expansion to registry loading**

Update `LoadRegistry` to expand `${VAR}` in env values:

```go
// In registry.go, after unmarshaling each entry:
import "os"
import "regexp"
import "strings"

var envVarRe = regexp.MustCompile(`\$\{(\w+)\}`)

func expandEnv(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.Trim(match, "${}")
		return os.Getenv(varName)
	})
}
```

Apply `expandEnv` to each value in `entry.Env` and `entry.Headers` after unmarshaling.

- [ ] **Step 3: Build and test**

Run: `go build ./... && go test ./internal/mcp/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: wire MCP manager to registry with ConnectEnabled"
```

---

## Phase 3: Skills Registry

### Task 6: Skills Registry — Multi-Format Loading

**Files:**
- Rewrite: `internal/skills/skill.go`
- Create: `internal/skills/skill_test.go`
- Modify: `internal/skills/executor.go`

- [ ] **Step 1: Write failing tests for multi-format skill loading**

```go
// internal/skills/skill_test.go
package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAMLSkill(t *testing.T) {
	dir := t.TempDir()
	content := `name: code-review
description: Review code changes
trigger: review
steps:
  - prompt: "Review: {{input}}"
    output: review_result
`
	path := filepath.Join(dir, "code-review.yaml")
	os.WriteFile(path, []byte(content), 0644)

	skill, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "code-review" {
		t.Errorf("Name = %q, want code-review", skill.Name)
	}
	if skill.Format != FormatYAML {
		t.Errorf("Format = %v, want FormatYAML", skill.Format)
	}
	if len(skill.Steps) != 1 {
		t.Errorf("Steps count = %d, want 1", len(skill.Steps))
	}
}

func TestLoadMarkdownSkill(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: debugging
description: Systematic debugging
trigger: debug
---

# Debugging Skill

When encountering a bug:
1. Reproduce
2. Isolate
3. Fix
`
	path := filepath.Join(dir, "debugging.md")
	os.WriteFile(path, []byte(content), 0644)

	skill, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "debugging" {
		t.Errorf("Name = %q, want debugging", skill.Name)
	}
	if skill.Format != FormatMarkdown {
		t.Errorf("Format = %v, want FormatMarkdown", skill.Format)
	}
	if skill.Content == "" {
		t.Error("Content should not be empty for markdown skill")
	}
}

func TestLoadDirectorySkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "brainstorming")
	os.MkdirAll(skillDir, 0755)

	content := `---
name: brainstorming
description: Brainstorm ideas
trigger: brainstorm
---

# Brainstorming

Ask questions one at a time.
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

	skill, err := LoadDir(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "brainstorming" {
		t.Errorf("Name = %q, want brainstorming", skill.Name)
	}
	if skill.Format != FormatDirectory {
		t.Errorf("Format = %v, want FormatDirectory", skill.Format)
	}
}

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()
	// YAML skill
	os.WriteFile(filepath.Join(dir, "review.yaml"), []byte("name: review\ndescription: Review\ntrigger: review\nsteps:\n  - prompt: test\n"), 0644)
	// Markdown skill
	os.WriteFile(filepath.Join(dir, "debug.md"), []byte("---\nname: debug\ndescription: Debug\ntrigger: debug\n---\n\n# Debug"), 0644)
	// Directory skill
	skillDir := filepath.Join(dir, "brain")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: brain\ndescription: Brain\ntrigger: brain\n---\n\n# Brain"), 0644)

	skills, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 3 {
		t.Errorf("expected 3 skills, got %d", len(skills))
	}
}

func TestLoadByName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "review.yaml"), []byte("name: review\ndescription: Review\ntrigger: review\nsteps:\n  - prompt: test\n"), 0644)

	skill, err := LoadByName(dir, "review")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "review" {
		t.Errorf("Name = %q, want review", skill.Name)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/skills/ -v`
Expected: FAIL

- [ ] **Step 3: Rewrite skills/skill.go with multi-format support**

```go
// internal/skills/skill.go
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatYAML     Format = "yaml"
	FormatMarkdown Format = "markdown"
	FormatDirectory Format = "directory"
)

type Skill struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Trigger     string `yaml:"trigger" json:"trigger"`
	Format      Format `yaml:"-" json:"format"`
	Steps       []Step `yaml:"steps,omitempty" json:"steps,omitempty"`
	Content     string `yaml:"-" json:"content,omitempty"`
	Dir         string `yaml:"-" json:"-"`
}

type Step struct {
	Prompt string         `yaml:"prompt" json:"prompt"`
	Tool   string         `yaml:"tool,omitempty" json:"tool,omitempty"`
	Args   map[string]any `yaml:"args,omitempty" json:"args,omitempty"`
	Output string         `yaml:"output,omitempty" json:"output,omitempty"`
}

type markdownFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Trigger     string `yaml:"trigger"`
}

func LoadFile(path string) (*Skill, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return loadYAML(path)
	case ".md":
		return loadMarkdown(path)
	default:
		return nil, fmt.Errorf("unsupported skill file format: %s", ext)
	}
}

func loadYAML(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}
	var skill Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("parse skill %s: %w", path, err)
	}
	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	skill.Format = FormatYAML
	return &skill, nil
}

func loadMarkdown(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}

	content := string(data)
	skill := &Skill{Format: FormatMarkdown}

	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end >= 0 {
			fm := content[3 : end+3]
			var meta markdownFrontmatter
			if err := yaml.Unmarshal([]byte(fm), &meta); err == nil {
				skill.Name = meta.Name
				skill.Description = meta.Description
				skill.Trigger = meta.Trigger
			}
			skill.Content = strings.TrimSpace(content[end+6:])
		} else {
			skill.Content = content
		}
	} else {
		skill.Content = content
	}

	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	return skill, nil
}

func LoadDir(dir string) (*Skill, error) {
	mdPath := filepath.Join(dir, "SKILL.md")
	skill, err := loadMarkdown(mdPath)
	if err != nil {
		return nil, fmt.Errorf("load skill dir %s: %w", dir, err)
	}
	skill.Format = FormatDirectory
	skill.Dir = dir
	return skill, nil
}

func LoadRegistry(dir string) (map[string]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Skill), nil
		}
		return nil, err
	}
	skills := make(map[string]*Skill)
	for _, entry := range entries {
		var skill *Skill
		var err error
		if entry.IsDir() {
			sub := filepath.Join(dir, entry.Name())
			if _, err := os.Stat(filepath.Join(sub, "SKILL.md")); err == nil {
				skill, err = LoadDir(sub)
			}
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".yaml" || ext == ".yml" || ext == ".md" {
				skill, err = LoadFile(filepath.Join(dir, entry.Name()))
			}
		}
		if err != nil {
			continue
		}
		if skill != nil {
			skills[skill.Name] = skill
		}
	}
	return skills, nil
}

func LoadByName(registryDir string, name string) (*Skill, error) {
	yamlPath := filepath.Join(registryDir, name+".yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return loadYAML(yamlPath)
	}
	mdPath := filepath.Join(registryDir, name+".md")
	if _, err := os.Stat(mdPath); err == nil {
		return loadMarkdown(mdPath)
	}
	dirPath := filepath.Join(registryDir, name)
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		if _, err := os.Stat(filepath.Join(dirPath, "SKILL.md")); err == nil {
			return LoadDir(dirPath)
		}
	}
	return nil, fmt.Errorf("skill %q not found in %s", name, registryDir)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/skills/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: multi-format skill loading (YAML, Markdown, Directory)"
```

---

## Phase 4: Agent Config

### Task 7: Agent Config Type & Loading

**Files:**
- Create: `internal/agent/config.go`
- Create: `internal/agent/config_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/agent/config_test.go
package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAgentConfig(t *testing.T) {
	dir := t.TempDir()
	content := `name: main
model: openai:gpt-4o
system_prompt: SOUL.md
enabled_mcp:
  - github
enabled_skills:
  - code-review
max_turns: 50
`
	path := filepath.Join(dir, "main.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadAgentConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "main" {
		t.Errorf("Name = %q, want main", cfg.Name)
	}
	if cfg.Model != "openai:gpt-4o" {
		t.Errorf("Model = %q, want openai:gpt-4o", cfg.Model)
	}
	if len(cfg.EnabledMCP) != 1 || cfg.EnabledMCP[0] != "github" {
		t.Errorf("EnabledMCP = %v, want [github]", cfg.EnabledMCP)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", cfg.MaxTurns)
	}
}

func TestLoadAgentConfigsFromDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.yaml"), []byte("name: main\nmodel: openai:gpt-4o\n"), 0644)
	os.WriteFile(filepath.Join(dir, "reviewer.yaml"), []byte("name: reviewer\nmodel: anthropic:claude-sonnet-4\n"), 0644)

	configs, err := LoadAgentConfigs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Errorf("expected 2 configs, got %d", len(configs))
	}
	if configs["main"].Model != "openai:gpt-4o" {
		t.Error("main agent model mismatch")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run TestLoadAgent -v`
Expected: FAIL

- [ ] **Step 3: Implement agent config**

```go
// internal/agent/config.go
package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	Name         string   `yaml:"name"`
	Model        string   `yaml:"model"`
	SystemPrompt string   `yaml:"system_prompt"`
	EnabledMCP   []string `yaml:"enabled_mcp,omitempty"`
	EnabledSkills []string `yaml:"enabled_skills,omitempty"`
	MaxTurns     int      `yaml:"max_turns,omitempty"`

	Sandbox *AgentSandboxConfig `yaml:"sandbox,omitempty"`
}

type AgentSandboxConfig struct {
	Root            string   `yaml:"root,omitempty"`
	AllowPaths      []string `yaml:"allow_paths,omitempty"`
	BlockedCommands []string `yaml:"blocked_commands,omitempty"`
}

func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config %s: %w", path, err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent config %s: %w", path, err)
	}
	if cfg.Name == "" {
		cfg.Name = strings.TrimSuffix(filepath.Base(path), ".yaml")
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 50
	}
	return &cfg, nil
}

func LoadAgentConfigs(dir string) (map[string]*AgentConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*AgentConfig), nil
		}
		return nil, err
	}
	configs := make(map[string]*AgentConfig)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		cfg, err := LoadAgentConfig(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		configs[cfg.Name] = cfg
	}
	return configs, nil
}
```

Add `"strings"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -run TestLoadAgent -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add per-workspace agent config loading"
```

---

## Phase 5: Sandbox Enforcement

### Task 8: Sandbox-Aware Filesystem Tools

**Files:**
- Modify: `internal/tools/builtin/filesystem.go`
- Create: `internal/tools/builtin/sandbox_test.go`

- [ ] **Step 1: Write failing tests for sandbox enforcement**

```go
// internal/tools/builtin/sandbox_test.go
package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileTool_SandboxAllowed(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	tool := NewReadFileTool(WithSandbox(dir))
	args, _ := json.Marshal(map[string]string{"path": testFile})
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Errorf("got %q, want hello", out)
	}
}

func TestReadFileTool_SandboxBlocked(t *testing.T) {
	dir := t.TempDir()
	tool := NewReadFileTool(WithSandbox(dir))
	args, _ := json.Marshal(map[string]string{"path": "/etc/passwd"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for path outside sandbox")
	}
}

func TestWriteFileTool_SandboxBlocked(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFileTool(WithSandbox(dir))
	args, _ := json.Marshal(map[string]string{"path": "/tmp/evil.txt", "content": "bad"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for write outside sandbox")
	}
}

func TestShellExecTool_SandboxEnforced(t *testing.T) {
	dir := t.TempDir()
	tool := NewShellExecTool(
		WithShellSandbox(dir, []string{"rm -rf /"}),
	)
	args, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for blocked command")
	}
}

func TestShellExecTool_Workdir(t *testing.T) {
	dir := t.TempDir()
	tool := NewShellExecTool(WithShellSandbox(dir, nil))
	args, _ := json.Marshal(map[string]string{"command": "pwd"})
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(dir)
	if out != abs+"\n" {
		t.Errorf("pwd = %q, want %q", out, abs)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tools/builtin/ -v`
Expected: FAIL — `WithSandbox`, `WithShellSandbox` not defined

- [ ] **Step 3: Add sandbox option to filesystem tools**

```go
// Add to internal/tools/builtin/filesystem.go

type SandboxChecker interface {
	IsAllowed(path string, write bool) bool
}

type ReadFileTool struct {
	sandbox SandboxChecker
}

type ReadFileToolOption func(*ReadFileTool)

func WithReadSandbox(s SandboxChecker) ReadFileToolOption {
	return func(t *ReadFileTool) { t.sandbox = s }
}

func NewReadFileTool(opts ...ReadFileToolOption) *ReadFileTool {
	t := &ReadFileTool{}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Update Execute:
func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if t.sandbox != nil && !t.sandbox.IsAllowed(a.Path, false) {
		return "", fmt.Errorf("path %q is outside allowed workspace paths", a.Path)
	}
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Same pattern for WriteFileTool with write=true sandbox check:
type WriteFileTool struct {
	sandbox SandboxChecker
}

type WriteFileToolOption func(*WriteFileTool)

func WithWriteSandbox(s SandboxChecker) WriteFileToolOption {
	return func(t *WriteFileTool) { t.sandbox = s }
}

func NewWriteFileTool(opts ...WriteFileToolOption) *WriteFileTool {
	t := &WriteFileTool{}
	for _, o := range opts {
		o(t)
	}
	return t
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if t.sandbox != nil && !t.sandbox.IsAllowed(a.Path, true) {
		return "", fmt.Errorf("path %q is outside allowed workspace paths (write)", a.Path)
	}
	if err := os.WriteFile(a.Path, []byte(a.Content), 0644); err != nil {
		return "", err
	}
	return "ok", nil
}
```

Add `"fmt"` to imports.

- [ ] **Step 4: Add sandbox option to shell tool**

```go
// Add to internal/tools/builtin/shell.go

type ShellExecTool struct {
	workdir         string
	blockedCommands []string
}

type ShellExecToolOption func(*ShellExecTool)

func WithShellSandbox(workdir string, blocked []string) ShellExecToolOption {
	return func(t *ShellExecTool) {
		t.workdir = workdir
		t.blockedCommands = blocked
	}
}

func NewShellExecTool(opts ...ShellExecToolOption) *ShellExecTool {
	t := &ShellExecTool{}
	for _, o := range opts {
		o(t)
	}
	return t
}

func (t *ShellExecTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a shellExecArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}

	cmd := a.Command
	for _, blocked := range t.blockedCommands {
		if strings.Contains(cmd, blocked) {
			return "", fmt.Errorf("command blocked by sandbox policy: %s", blocked)
		}
	}

	execCmd := exec.CommandContext(ctx, "sh", "-c", cmd)
	if t.workdir != "" {
		execCmd.Dir = t.workdir
	} else if a.Dir != "" {
		execCmd.Dir = a.Dir
	}
	out, err := execCmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}
```

Add `"fmt"` and `"strings"` to imports.

- [ ] **Step 5: Add convenience constructors for tests**

```go
// Add a simple sandbox adapter and convenience wrappers:

type simpleSandbox struct {
	allowPaths []string
}

func (s *simpleSandbox) IsAllowed(path string, write bool) bool {
	abs, _ := filepath.Abs(path)
	for _, p := range s.allowPaths {
		ap, _ := filepath.Abs(p)
		rel, err := filepath.Rel(ap, abs)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

func WithSandbox(allowedPath string) ReadFileToolOption {
	s := &simpleSandbox{allowPaths: []string{allowedPath}}
	return WithReadSandbox(s)
}

// For WriteFileTool:
func WithWriteSandboxPath(allowedPath string) WriteFileToolOption {
	s := &simpleSandbox{allowPaths: []string{allowedPath}}
	return WithWriteSandbox(s)
}
```

Update test to use `WithSandbox` for ReadFileTool and `WithWriteSandboxPath` for WriteFileTool.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/tools/builtin/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: add sandbox enforcement to filesystem and shell tools"
```

---

## Phase 6: Startup Wiring

### Task 9: Rewrite Startup Pipeline — helpers.go & root.go

**Files:**
- Rewrite: `cmd/cobot/helpers.go`
- Modify: `cmd/cobot/root.go`

This task wires everything together: workspace resolution → MCP registry → skills loading → agent creation → sandbox setup.

- [ ] **Step 1: Add --data flag to root.go**

```go
// In cmd/cobot/root.go init(), add:
rootCmd.PersistentFlags().StringVar(&dataPath, "data", "", "data directory (default: $COBOT_DATA_PATH or ~/.local/share/cobot)")
```

Add `var dataPath string` alongside existing vars.

In `loadConfig()`, at the beginning:
```go
if dataPath != "" {
    os.Setenv("COBOT_DATA_PATH", dataPath)
}
if cfgPath != "" {
    os.Setenv("COBOT_CONFIG_PATH", filepath.Dir(cfgPath))
}
```

- [ ] **Step 2: Rewrite loadConfig to use new workspace system**

```go
// In cmd/cobot/root.go, rewrite loadConfig:
func loadConfig() (*cobot.Config, error) {
    cfg := cobot.DefaultConfig()

    if cfgPath != "" {
        if err := config.LoadFromFile(cfg, cfgPath); err != nil {
            return nil, fmt.Errorf("load config: %w", err)
        }
    } else {
        globalCfg := xdg.GlobalConfigPath()
        if _, err := os.Stat(globalCfg); err == nil {
            if err := config.LoadFromFile(cfg, globalCfg); err != nil {
                return nil, fmt.Errorf("load global config: %w", err)
            }
        }
    }

    config.ApplyEnvVars(cfg)

    if modelName != "" {
        cfg.Model = modelName
    }

    return cfg, nil
}
```

- [ ] **Step 3: Rewrite helpers.go with new initialization**

```go
// cmd/cobot/helpers.go — rewrite initAgent:
func initAgent(cfg *cobot.Config) (*agent.Agent, *workspace.Workspace, error) {
    wsMgr, err := workspace.NewManager()
    if err != nil {
        return nil, nil, fmt.Errorf("create workspace manager: %w", err)
    }

    wsName := cfg.Workspace
    ws, err := wsMgr.ResolveByNameOrDiscover(wsName, ".")
    if err != nil {
        return nil, nil, fmt.Errorf("resolve workspace: %w", err)
    }
    if err := ws.EnsureDirs(); err != nil {
        return nil, nil, err
    }

    agentCfg, err := resolveAgentConfig(ws, cfg)
    if err != nil {
        return nil, nil, err
    }

    a := agent.New(cfg)
    a.SetToolRegistry(tools.NewRegistry())

    provider, err := initProvider(cfg)
    if err != nil {
        return nil, nil, err
    }
    a.SetProvider(provider)

    registerBuiltinTools(a, ws)

    if err := connectMCPServers(a, ws); err != nil {
        fmt.Fprintf(os.Stderr, "warning: MCP connection error: %v\n", err)
    }

    if err := loadSkills(a, ws); err != nil {
        fmt.Fprintf(os.Stderr, "warning: skills loading error: %v\n", err)
    }

    return a, ws, nil
}

func resolveAgentConfig(ws *workspace.Workspace, cfg *cobot.Config) (*agent.AgentConfig, error) {
    agentsDir := ws.AgentsDir()
    configs, err := agent.LoadAgentConfigs(agentsDir)
    if err != nil || len(configs) == 0 {
        return &agent.AgentConfig{
            Name:  "main",
            Model: cfg.Model,
            MaxTurns: cfg.MaxTurns,
        }, nil
    }
    defaultName := ws.Config.DefaultAgent
    if defaultName == "" {
        defaultName = "main"
    }
    if ac, ok := configs[defaultName]; ok {
        if ac.Model != "" {
            cfg.Model = ac.Model
        }
        return ac, nil
    }
    for _, ac := range configs {
        if ac.Model != "" {
            cfg.Model = ac.Model
        }
        return ac, nil
    }
    return &agent.AgentConfig{Name: "main", Model: cfg.Model, MaxTurns: cfg.MaxTurns}, nil
}

func registerBuiltinTools(a *agent.Agent, ws *workspace.Workspace) {
    sandbox := ws.Config.Sandbox
    sandboxChecker := &builtin.WorkspaceSandbox{Root: sandbox.Root, AllowPaths: sandbox.AllowPaths}

    a.RegisterTool(builtin.NewReadFileTool(builtin.WithReadSandbox(sandboxChecker)))
    a.RegisterTool(builtin.NewWriteFileTool(builtin.WithWriteSandbox(sandboxChecker)))
    a.RegisterTool(builtin.NewShellExecTool(builtin.WithShellSandbox(
        sandbox.Root,
        sandbox.BlockedCommands,
    )))
}

func connectMCPServers(a *agent.Agent, ws *workspace.Workspace) error {
    if len(ws.Config.EnabledMCP) == 0 {
        return nil
    }
    registry, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
    if err != nil {
        return err
    }
    mgr := mcp.NewMCPManager()
    if err := mgr.ConnectEnabled(context.Background(), registry, ws.Config.EnabledMCP); err != nil {
        return err
    }
    for _, name := range ws.Config.EnabledMCP) {
        adapters, err := mgr.ToolAdapters(context.Background(), name)
        if err != nil {
            continue
        }
        for _, adapter := range adapters {
            a.RegisterTool(adapter)
        }
    }
    return nil
}

func loadSkills(a *agent.Agent, ws *workspace.Workspace) error {
    globalSkills, err := skills.LoadRegistry(xdg.SkillsRegistryDir())
    if err != nil {
        return err
    }
    wsSkills, err := skills.LoadRegistry(ws.SkillsDir())
    if err != nil {
        return err
    }

    all := make(map[string]*skills.Skill)
    for name, s := range globalSkills {
        all[name] = s
    }
    for name, s := range wsSkills {
        all[name] = s
    }

    for _, s := range all {
        a.RegisterSkill(s)
    }
    return nil
}
```

Add `"context"`, `"fmt"`, `"os"` imports, and the new package imports (`mcp`, `skills`, `builtin`, `workspace`, `agent`, `tools`, `xdg`).

- [ ] **Step 4: Add WorkspaceSandbox adapter**

```go
// Add to internal/tools/builtin/sandbox.go (new file):
package builtin

import (
    "path/filepath"
    "strings"
)

type WorkspaceSandbox struct {
    Root       string
    AllowPaths []string
}

func (s *WorkspaceSandbox) IsAllowed(path string, write bool) bool {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return false
    }
    if s.Root != "" {
        absRoot, _ := filepath.Abs(s.Root)
        if rel, err := filepath.Rel(absRoot, absPath); err == nil && !strings.HasPrefix(rel, "..") {
            return true
        }
    }
    for _, p := range s.AllowPaths {
        absP, _ := filepath.Abs(p)
        if rel, err := filepath.Rel(absP, absPath); err == nil && !strings.HasPrefix(rel, "..") {
            return true
        }
    }
    return false
}
```

- [ ] **Step 5: Add RegisterSkill to Agent**

```go
// Add to internal/agent/agent.go:
func (a *Agent) RegisterSkill(s *skills.Skill) {
    a.RegisterTool(&SkillTool{skill: s})
}
```

And create `internal/agent/skill_tool.go`:
```go
package agent

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/cobot-agent/cobot/internal/skills"
    cobot "github.com/cobot-agent/cobot/pkg"
)

type SkillTool struct {
    skill *skills.Skill
}

func (t *SkillTool) Name() string        { return "skill_" + t.skill.Name }
func (t *SkillTool) Description() string { return t.skill.Description }
func (t *SkillTool) Parameters() json.RawMessage {
    return json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}}}`)
}
func (t *SkillTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var a struct{ Input string `json:"input"` }
    if err := json.Unmarshal(args, &a); err != nil {
        return "", err
    }
    if t.skill.Format == skills.FormatYAML && len(t.skill.Steps) > 0 {
        return fmt.Sprintf("Skill %q triggered with input: %s", t.skill.Name, a.Input), nil
    }
    return t.skill.Content, nil
}

var _ cobot.Tool = (*SkillTool)(nil)
```

- [ ] **Step 6: Build and fix all compilation errors**

Run: `go build ./...`

This is the integration step — fix all type mismatches, missing imports, and broken callers.

Key files that will need updates:
- `cmd/cobot/chat.go` — update `initAgent()` call signature
- `cmd/cobot/tui.go` — update `initAgent()` call signature
- `cmd/cobot/setup.go` — update workspace creation

- [ ] **Step 7: Run all tests**

Run: `go test ./...`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: wire startup pipeline with workspace, MCP registry, skills, sandbox"
```

---

## Phase 7: CLI Commands

### Task 10: MCP CLI Commands

**Files:**
- Create: `cmd/cobot/mcp_cmd.go`

- [ ] **Step 1: Implement MCP CLI commands**

```go
// cmd/cobot/mcp_cmd.go
package main

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
    "github.com/cobot-agent/cobot/internal/mcp"
    "github.com/cobot-agent/cobot/internal/xdg"
)

var mcpCmd = &cobra.Command{
    Use:   "mcp",
    Short: "Manage MCP server registry",
}

var mcpListCmd = &cobra.Command{
    Use:   "list",
    Short: "List registered MCP servers",
    RunE: func(cmd *cobra.Command, args []string) error {
        registry, err := mcp.LoadRegistry(xdg.MCPRegistryDir())
        if err != nil {
            return err
        }
        if len(registry) == 0 {
            fmt.Println("No MCP servers registered.")
            return nil
        }
        for name, entry := range registry {
            fmt.Printf("%s\t%s\t%s\n", name, entry.Transport, entry.Description)
        }
        return nil
    },
}

var mcpAddCmd = &cobra.Command{
    Use:   "add <name> -f <file>",
    Short: "Register an MCP server from a YAML file",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := args[0]
        file, _ := cmd.Flags().GetString("file")
        if file == "" {
            return fmt.Errorf("required flag: --file")
        }
        destDir := xdg.MCPRegistryDir()
        if err := os.MkdirAll(destDir, 0755); err != nil {
            return err
        }
        dest := filepath.Join(destDir, name+".yaml")
        data, err := os.ReadFile(file)
        if err != nil {
            return err
        }
        return os.WriteFile(dest, data, 0644)
    },
}

var mcpRemoveCmd = &cobra.Command{
    Use:   "remove <name>",
    Short: "Unregister an MCP server",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        path := filepath.Join(xdg.MCPRegistryDir(), args[0]+".yaml")
        if err := os.Remove(path); err != nil {
            return fmt.Errorf("remove MCP server %q: %w", args[0], err)
        }
        fmt.Printf("Removed MCP server %q\n", args[0])
        return nil
    },
}

var mcpShowCmd = &cobra.Command{
    Use:   "show <name>",
    Short: "Show MCP server details",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        path := filepath.Join(xdg.MCPRegistryDir(), args[0]+".yaml")
        data, err := os.ReadFile(path)
        if err != nil {
            return fmt.Errorf("MCP server %q not found", args[0])
        }
        fmt.Println(string(data))
        return nil
    },
}

func init() {
    mcpAddCmd.Flags().StringP("file", "f", "", "YAML file to register")
    mcpCmd.AddCommand(mcpListCmd, mcpAddCmd, mcpRemoveCmd, mcpShowCmd)
    rootCmd.AddCommand(mcpCmd)
}
```

- [ ] **Step 2: Build and test**

Run: `go build ./... && ./build/cobot mcp list`
Expected: Builds, prints "No MCP servers registered." or lists servers

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add MCP management CLI commands (list/add/remove/show)"
```

---

### Task 11: Skill CLI Commands

**Files:**
- Create: `cmd/cobot/skill_cmd.go`

- [ ] **Step 1: Implement skill CLI commands**

```go
// cmd/cobot/skill_cmd.go
package main

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
    "github.com/cobot-agent/cobot/internal/skills"
    "github.com/cobot-agent/cobot/internal/xdg"
)

var skillCmd = &cobra.Command{
    Use:   "skill",
    Short: "Manage skills",
}

var skillListCmd = &cobra.Command{
    Use:   "list",
    Short: "List skills",
    RunE: func(cmd *cobra.Command, args []string) error {
        scope, _ := cmd.Flags().GetString("scope")
        var dir string
        if scope == "workspace" {
            wsMgr, err := workspace.NewManager()
            if err != nil {
                return err
            }
            ws, err := wsMgr.ResolveByNameOrDiscover("", ".")
            if err != nil {
                return err
            }
            dir = ws.SkillsDir()
        } else {
            dir = xdg.SkillsRegistryDir()
        }

        reg, err := skills.LoadRegistry(dir)
        if err != nil {
            return err
        }
        if len(reg) == 0 {
            fmt.Println("No skills found.")
            return nil
        }
        for name, s := range reg {
            fmt.Printf("%s\t%s\t%s\n", name, s.Format, s.Description)
        }
        return nil
    },
}

var skillAddCmd = &cobra.Command{
    Use:   "add <name> -f <file>",
    Short: "Add a skill to the global registry",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        file, _ := cmd.Flags().GetString("file")
        if file == "" {
            return fmt.Errorf("required flag: --file")
        }
        ext := filepath.Ext(file)
        destDir := xdg.SkillsRegistryDir()
        if err := os.MkdirAll(destDir, 0755); err != nil {
            return err
        }
        dest := filepath.Join(destDir, args[0]+ext)
        data, err := os.ReadFile(file)
        if err != nil {
            return err
        }
        return os.WriteFile(dest, data, 0644)
    },
}

var skillRemoveCmd = &cobra.Command{
    Use:   "remove <name>",
    Short: "Remove a skill from the global registry",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        dir := xdg.SkillsRegistryDir()
        for _, ext := range []string{".yaml", ".md"} {
            path := filepath.Join(dir, args[0]+ext)
            if _, err := os.Stat(path); err == nil {
                return os.Remove(path)
            }
        }
        dirPath := filepath.Join(dir, args[0])
        if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
            return os.RemoveAll(dirPath)
        }
        return fmt.Errorf("skill %q not found", args[0])
    },
}

func init() {
    skillListCmd.Flags().String("scope", "global", "List skills from: global|workspace")
    skillCmd.AddCommand(skillListCmd, skillAddCmd, skillRemoveCmd)
    rootCmd.AddCommand(skillCmd)
}
```

- [ ] **Step 2: Build and test**

Run: `go build ./... && ./build/cobot skill list`
Expected: Builds, runs

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add skill management CLI commands (list/add/remove)"
```

---

### Task 12: Update Workspace CLI Commands

**Files:**
- Rewrite: `cmd/cobot/workspace.go`

- [ ] **Step 1: Rewrite workspace CLI for new manager**

Remove `switch`/`current` commands. Add workspace path support. Update `create` to accept `--root` and `--path` flags.

```go
// cmd/cobot/workspace.go — full rewrite
package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/cobot-agent/cobot/internal/workspace"
)

var (
    wsRoot string
    wsPath string
)

var workspaceCmd = &cobra.Command{
    Use:   "workspace",
    Short: "Manage workspaces",
    Aliases: []string{"ws"},
}

var wsListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all workspaces",
    Aliases: []string{"ls"},
    RunE: func(cmd *cobra.Command, args []string) error {
        m, err := workspace.NewManager()
        if err != nil {
            return err
        }
        defs, err := m.List()
        if err != nil {
            return err
        }
        for _, d := range defs {
            path := d.ResolvePath("")
            fmt.Printf("%s\t%s\t%s\n", d.Name, d.Type, path)
        }
        return nil
    },
}

var wsCreateCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Create a new workspace",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        m, err := workspace.NewManager()
        if err != nil {
            return err
        }
        wsType := workspace.WorkspaceTypeCustom
        if wsRoot != "" {
            wsType = workspace.WorkspaceTypeProject
        }
        ws, err := m.Create(args[0], wsType, wsRoot, wsPath)
        if err != nil {
            return err
        }
        fmt.Printf("Created workspace %q (data: %s)\n", args[0], ws.DataDir)
        return nil
    },
}

var wsDeleteCmd = &cobra.Command{
    Use:   "delete <name>",
    Short: "Delete a workspace",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        m, err := workspace.NewManager()
        if err != nil {
            return err
        }
        return m.Delete(args[0])
    },
}

var wsShowCmd = &cobra.Command{
    Use:   "show [name]",
    Short: "Show workspace configuration",
    RunE: func(cmd *cobra.Command, args []string) error {
        m, err := workspace.NewManager()
        if err != nil {
            return err
        }
        name := "default"
        if len(args) > 0 {
            name = args[0]
        }
        ws, err := m.Resolve(name)
        if err != nil {
            return err
        }
        fmt.Printf("Name: %s\n", ws.Definition.Name)
        fmt.Printf("Type: %s\n", ws.Definition.Type)
        fmt.Printf("Data: %s\n", ws.DataDir)
        if ws.Definition.Root != "" {
            fmt.Printf("Root: %s\n", ws.Definition.Root)
        }
        fmt.Printf("Enabled MCP: %v\n", ws.Config.EnabledMCP)
        fmt.Printf("Enabled Skills: %v\n", ws.Config.EnabledSkills)
        fmt.Printf("Agents: %v\n", ws.Config.Agents)
        return nil
    },
}

func init() {
    wsCreateCmd.Flags().StringVar(&wsRoot, "root", "", "Project root directory")
    wsCreateCmd.Flags().StringVar(&wsPath, "path", "", "Custom data directory path")
    workspaceCmd.AddCommand(wsListCmd, wsCreateCmd, wsDeleteCmd, wsShowCmd)
    rootCmd.AddCommand(workspaceCmd)
}
```

- [ ] **Step 2: Build and test**

Run: `go build ./... && ./build/cobot workspace list`
Expected: Lists default workspace

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "refactor: rewrite workspace CLI with new manager (no current tracking)"
```

---

## Phase 8: Config Env Var Updates

### Task 13: Add COBOT_CONFIG_PATH / COBOT_DATA_PATH to ApplyEnvVars

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add new env var handling**

```go
// In internal/config/config.go, add to ApplyEnvVars:
func ApplyEnvVars(cfg *cobot.Config) {
    if v := os.Getenv("COBOT_MODEL"); v != "" {
        cfg.Model = v
    }
    if v := os.Getenv("COBOT_WORKSPACE"); v != "" {
        cfg.Workspace = v
    }
    if v := os.Getenv("COBOT_CONFIG_PATH"); v != "" {
        cfg.ConfigPath = v
    }
    if v := os.Getenv("COBOT_DATA_PATH"); v != "" {
        cfg.DataPath = v
    }
    if v := os.Getenv("OPENAI_API_KEY"); v != "" {
        if cfg.APIKeys == nil {
            cfg.APIKeys = make(map[string]string)
        }
        cfg.APIKeys["openai"] = v
    }
    if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
        if cfg.APIKeys == nil {
            cfg.APIKeys = make(map[string]string)
        }
        cfg.APIKeys["anthropic"] = v
    }
}
```

- [ ] **Step 2: Build and test**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add COBOT_CONFIG_PATH and COBOT_DATA_PATH env var support"
```

---

## Phase 9: ACP API Workspace Support

### Task 14: Add workspace parameter to ACP session/new

**Files:**
- Modify: `api/acp/types.go`
- Modify: `internal/acp/server.go`

- [ ] **Step 1: Update ACP types**

Add `Workspace` and `Agent` fields to `NewSessionRequest`:

```go
// In api/acp/types.go, update NewSessionRequest:
type NewSessionRequest struct {
    Message    string     `json:"message"`
    Workspace  string     `json:"workspace,omitempty"`
    Agent      string     `json:"agent,omitempty"`
    MCPServers []MCPServer `json:"mcp_servers,omitempty"`
}
```

- [ ] **Step 2: Update ACP server to resolve workspace**

In `internal/acp/server.go`, update `handleSessionNew` to use workspace manager to resolve the workspace and agent name.

- [ ] **Step 3: Build and test**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: add workspace and agent params to ACP session/new"
```

---

## Phase 10: Cleanup

### Task 15: Remove Dead Code & Final Verification

**Files:**
- Remove: any unused files from old workspace system
- Clean: all compilation warnings

- [ ] **Step 1: Find and remove dead code**

Run: `go vet ./...`

Remove:
- Old `ManagerConfig` type if no longer used
- Old `managerConfigPath()` function
- Any files still referencing old workspace structure

- [ ] **Step 2: Run full build and test suite**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass, no warnings

- [ ] **Step 3: Manual smoke test**

```bash
./build/cobot workspace list
./build/cobot mcp list
./build/cobot skill list
./build/cobot workspace show default
./build/cobot workspace create test-ws
./build/cobot workspace list
./build/cobot workspace delete test-ws
```

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "chore: cleanup dead code from old config/workspace system"
```

---

## Self-Review

### Spec Coverage

| Spec Section | Task |
|---|---|
| Path Resolution | Task 1, Task 13 |
| Directory Structure | Task 1, Task 3 |
| Global Config format | Task 2 |
| MCP Registry format | Task 4 |
| MCP Manager integration | Task 5 |
| Skills Registry (YAML/MD/Dir) | Task 6 |
| Workspace Definition | Task 3 |
| Workspace Config | Task 3 |
| Agent Config | Task 7 |
| Workspace Selection | Task 3, Task 9 |
| Skills Loading priority | Task 9 |
| MCP Connection Lifecycle | Task 5, Task 9 |
| Sandbox Mechanism | Task 8 |
| Mutability Boundary | Task 2 (type level), Task 9 (enforcement) |
| CLI Commands (MCP) | Task 10 |
| CLI Commands (Skills) | Task 11 |
| CLI Commands (Workspace) | Task 12 |
| ACP API Changes | Task 14 |

### Placeholder Scan

No TBD, TODO, or placeholder patterns found.

### Type Consistency

All types defined in Task 2 (`SandboxConfig`) are used consistently in Tasks 3, 7, 8, 9. `WorkspaceDefinition` / `WorkspaceConfig` / `Workspace` defined in Task 3 used consistently in Tasks 7, 9, 10, 11, 12. `RegistryEntry` from Task 4 used in Task 5. `Skill` from Task 6 used in Task 9.
