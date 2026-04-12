package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlData := `name: test-skill
description: A test skill
trigger: /test
steps:
  - prompt: "Hello world"
    output: greeting
`
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte(yamlData), 0644)

	skill, err := LoadFromYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", skill.Name)
	}
	if skill.Trigger != "/test" {
		t.Errorf("expected /test, got %s", skill.Trigger)
	}
	if len(skill.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(skill.Steps))
	}
	if skill.Steps[0].Prompt != "Hello world" {
		t.Errorf("unexpected prompt: %s", skill.Steps[0].Prompt)
	}
	if skill.Steps[0].Output != "greeting" {
		t.Errorf("unexpected output: %s", skill.Steps[0].Output)
	}
}

func TestLoadFromYAMLNoName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myskill.yaml")
	os.WriteFile(path, []byte("trigger: /my\n"), 0644)

	skill, err := LoadFromYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "myskill" {
		t.Errorf("expected myskill, got %s", skill.Name)
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("name: a\ntrigger: /a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("name: b\ntrigger: /b\n"), 0644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("not a skill"), 0644)

	skills, err := LoadFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestLoadFromDirMissing(t *testing.T) {
	skills, err := LoadFromDir("/nonexistent")
	if err != nil {
		t.Fatal("expected nil error for missing dir")
	}
	if skills != nil {
		t.Error("expected nil skills")
	}
}

func TestLoadFileYAML(t *testing.T) {
	dir := t.TempDir()
	yamlData := `name: yaml-skill
description: A YAML skill
trigger: /yaml
steps:
  - prompt: "Step 1"
    output: out1
`
	path := filepath.Join(dir, "skill.yaml")
	os.WriteFile(path, []byte(yamlData), 0644)

	skill, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "yaml-skill" {
		t.Errorf("expected yaml-skill, got %s", skill.Name)
	}
	if skill.Format != FormatYAML {
		t.Errorf("expected FormatYAML, got %s", skill.Format)
	}
	if len(skill.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(skill.Steps))
	}
}

func TestLoadFileYML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "skill.yml"), []byte("name: yml-skill\ntrigger: /yml\n"), 0644)

	skill, err := LoadFile(filepath.Join(dir, "skill.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "yml-skill" {
		t.Errorf("expected yml-skill, got %s", skill.Name)
	}
	if skill.Format != FormatYAML {
		t.Errorf("expected FormatYAML, got %s", skill.Format)
	}
}

func TestLoadFileMarkdown(t *testing.T) {
	dir := t.TempDir()
	mdData := `---
name: debugging
description: Systematic debugging skill
trigger: debug
---

# Debugging Skill

Body content here...
`
	path := filepath.Join(dir, "debugging.md")
	os.WriteFile(path, []byte(mdData), 0644)

	skill, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "debugging" {
		t.Errorf("expected debugging, got %s", skill.Name)
	}
	if skill.Description != "Systematic debugging skill" {
		t.Errorf("unexpected description: %s", skill.Description)
	}
	if skill.Trigger != "debug" {
		t.Errorf("expected debug, got %s", skill.Trigger)
	}
	if skill.Format != FormatMarkdown {
		t.Errorf("expected FormatMarkdown, got %s", skill.Format)
	}
	if skill.Content == "" {
		t.Error("expected non-empty Content")
	}
}

func TestLoadDirSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0755)

	mdData := `---
name: dir-skill
description: A directory skill
trigger: /dir
---

# Directory Skill

Content from SKILL.md
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(mdData), 0644)

	skill, err := LoadDir(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "dir-skill" {
		t.Errorf("expected dir-skill, got %s", skill.Name)
	}
	if skill.Format != FormatDirectory {
		t.Errorf("expected FormatDirectory, got %s", skill.Format)
	}
	if skill.Dir != skillDir {
		t.Errorf("expected Dir=%s, got %s", skillDir, skill.Dir)
	}
	if skill.Content == "" {
		t.Error("expected non-empty Content")
	}
}

func TestLoadDirSkillNoSKILLMD(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "empty-skill")
	os.MkdirAll(skillDir, 0755)

	_, err := LoadDir(skillDir)
	if err == nil {
		t.Error("expected error for missing SKILL.md")
	}
}

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "yaml-skill.yaml"), []byte("name: yaml-skill\ntrigger: /yaml\n"), 0644)

	mdData := `---
name: md-skill
description: MD skill
trigger: /md
---

# MD Skill
`
	os.WriteFile(filepath.Join(dir, "md-skill.md"), []byte(mdData), 0644)

	skillDir := filepath.Join(dir, "dir-skill")
	os.MkdirAll(skillDir, 0755)
	dirMD := `---
name: dir-skill
description: Dir skill
trigger: /dir
---

# Dir Skill
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(dirMD), 0644)

	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a skill"), 0644)

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(registry))
	}
	if _, ok := registry["yaml-skill"]; !ok {
		t.Error("yaml-skill not found")
	}
	if _, ok := registry["md-skill"]; !ok {
		t.Error("md-skill not found")
	}
	if _, ok := registry["dir-skill"]; !ok {
		t.Error("dir-skill not found")
	}
	if registry["yaml-skill"].Format != FormatYAML {
		t.Error("yaml-skill should be FormatYAML")
	}
	if registry["md-skill"].Format != FormatMarkdown {
		t.Error("md-skill should be FormatMarkdown")
	}
	if registry["dir-skill"].Format != FormatDirectory {
		t.Error("dir-skill should be FormatDirectory")
	}
}

func TestLoadByName(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "my-skill.yaml"), []byte("name: my-skill\ntrigger: /yaml\n"), 0644)

	mdData := `---
name: other-skill
description: Another skill
trigger: /other
---

# Other
`
	os.WriteFile(filepath.Join(dir, "other-skill.md"), []byte(mdData), 0644)

	skillDir := filepath.Join(dir, "dir-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: dir-skill\ntrigger: /dir\n---\n\n# Dir\n"), 0644)

	s, err := LoadByName(dir, "my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "my-skill" {
		t.Errorf("expected my-skill, got %s", s.Name)
	}

	s, err = LoadByName(dir, "other-skill")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "other-skill" {
		t.Errorf("expected other-skill, got %s", s.Name)
	}

	s, err = LoadByName(dir, "dir-skill")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "dir-skill" {
		t.Errorf("expected dir-skill, got %s", s.Name)
	}

	_, err = LoadByName(dir, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestLoadFileUnsupportedExt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skill.txt")
	os.WriteFile(path, []byte("not a skill"), 0644)

	_, err := LoadFile(path)
	if err == nil {
		t.Error("expected error for unsupported extension")
	}
}
