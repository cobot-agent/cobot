package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
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

	skill, err := loadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", skill.Name)
	}
	if skill.Trigger != "/test" {
		t.Errorf("expected /test, got %s", skill.Trigger)
	}
	if skill.Format != FormatYAML {
		t.Errorf("expected yaml format, got %s", skill.Format)
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

func TestLoadYAMLNoName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myskill.yaml")
	os.WriteFile(path, []byte("trigger: /my\n"), 0644)

	skill, err := loadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "myskill" {
		t.Errorf("expected myskill, got %s", skill.Name)
	}
}

func TestLoadMarkdownWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mdData := `---
name: md-skill
description: A markdown skill
trigger: /md
---
# Markdown Skill

This is the body of the skill.
`
	path := filepath.Join(dir, "skill.md")
	os.WriteFile(path, []byte(mdData), 0644)

	skill, err := loadMarkdown(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "md-skill" {
		t.Errorf("expected md-skill, got %s", skill.Name)
	}
	if skill.Description != "A markdown skill" {
		t.Errorf("expected 'A markdown skill', got %s", skill.Description)
	}
	if skill.Trigger != "/md" {
		t.Errorf("expected /md, got %s", skill.Trigger)
	}
	if skill.Format != FormatMarkdown {
		t.Errorf("expected markdown format, got %s", skill.Format)
	}
	if skill.Content != "# Markdown Skill\n\nThis is the body of the skill." {
		t.Errorf("unexpected content: %q", skill.Content)
	}
}

func TestLoadMarkdownNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mdData := "# Just a skill\n\nPlain markdown content."
	path := filepath.Join(dir, "myskill.md")
	os.WriteFile(path, []byte(mdData), 0644)

	skill, err := loadMarkdown(path)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "myskill" {
		t.Errorf("expected name from filename 'myskill', got %s", skill.Name)
	}
	if skill.Format != FormatMarkdown {
		t.Errorf("expected markdown format, got %s", skill.Format)
	}
}

func TestLoadDirSkill(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "my-dir-skill")
	os.Mkdir(subdir, 0755)

	skillMd := `---
name: dir-skill
description: A directory skill
trigger: /dir
---
Directory skill body.
`
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte(skillMd), 0644)

	skill, err := LoadDir(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "dir-skill" {
		t.Errorf("expected dir-skill, got %s", skill.Name)
	}
	if skill.Format != FormatDirectory {
		t.Errorf("expected directory format, got %s", skill.Format)
	}
	if skill.Dir != subdir {
		t.Errorf("expected dir %s, got %s", subdir, skill.Dir)
	}
	if skill.Content != "Directory skill body." {
		t.Errorf("unexpected content: %q", skill.Content)
	}
}

func TestLoadDirWithScripts(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "scripted-skill")
	os.Mkdir(subdir, 0755)
	os.Mkdir(filepath.Join(subdir, "scripts"), 0755)

	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("---\nname: scripted\n---\nBody"), 0644)
	os.WriteFile(filepath.Join(subdir, "scripts", "setup.sh"), []byte("#!/bin/bash\necho setup"), 0755)
	os.WriteFile(filepath.Join(subdir, "scripts", "run.py"), []byte("print('run')"), 0644)

	skill, err := LoadDir(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skill.Steps) != 2 {
		t.Fatalf("expected 2 steps from scripts, got %d", len(skill.Steps))
	}

	names := map[string]bool{}
	for _, s := range skill.Steps {
		names[s.Args["file"].(string)] = true
	}
	if !names["setup.sh"] || !names["run.py"] {
		t.Errorf("expected setup.sh and run.py in steps, got %v", names)
	}
}

func TestLoadRegistryMixedFormats(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "yaml-skill.yaml"), []byte("name: yaml-one\ntrigger: /y\n"), 0644)
	os.WriteFile(filepath.Join(dir, "md-skill.md"), []byte("---\nname: md-one\ntrigger: /m\n---\nBody"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a skill"), 0644)

	subdir := filepath.Join(dir, "dir-skill")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("---\nname: dir-one\ntrigger: /d\n---\nDir body"), 0644)

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(registry) != 3 {
		t.Fatalf("expected 3 skills, got %d: %v", len(registry), registry)
	}
	if _, ok := registry["yaml-one"]; !ok {
		t.Error("missing yaml-one")
	}
	if _, ok := registry["md-one"]; !ok {
		t.Error("missing md-one")
	}
	if _, ok := registry["dir-one"]; !ok {
		t.Error("missing dir-one")
	}

	if registry["yaml-one"].Format != FormatYAML {
		t.Errorf("yaml-one format: %s", registry["yaml-one"].Format)
	}
	if registry["md-one"].Format != FormatMarkdown {
		t.Errorf("md-one format: %s", registry["md-one"].Format)
	}
	if registry["dir-one"].Format != FormatDirectory {
		t.Errorf("dir-one format: %s", registry["dir-one"].Format)
	}
}

func TestLoadRegistryMissing(t *testing.T) {
	registry, err := LoadRegistry("/nonexistent")
	if err != nil {
		t.Fatal("expected nil error for missing dir")
	}
	if registry != nil {
		t.Error("expected nil registry")
	}
}

func TestLoadByNameYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "my-skill.yaml"), []byte("name: my-skill\ntrigger: /my\n"), 0644)

	skill, err := LoadByName(dir, "my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "my-skill" {
		t.Errorf("expected my-skill, got %s", skill.Name)
	}
	if skill.Format != FormatYAML {
		t.Errorf("expected yaml, got %s", skill.Format)
	}
}

func TestLoadByNameMarkdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "my-skill.md"), []byte("---\nname: my-skill\n---\nBody"), 0644)

	skill, err := LoadByName(dir, "my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Format != FormatMarkdown {
		t.Errorf("expected markdown, got %s", skill.Format)
	}
}

func TestLoadByNameDirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "my-skill")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("---\nname: my-skill\n---\nBody"), 0644)

	skill, err := LoadByName(dir, "my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Format != FormatDirectory {
		t.Errorf("expected directory, got %s", skill.Format)
	}
}

func TestLoadByNameNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadByName(dir, "missing")
	if err == nil {
		t.Error("expected error for missing skill")
	}
}

func TestLoadFileAutoDetect(t *testing.T) {
	dir := t.TempDir()

	yamlPath := filepath.Join(dir, "test.yaml")
	os.WriteFile(yamlPath, []byte("name: yaml-test\ntrigger: /y\n"), 0644)

	mdPath := filepath.Join(dir, "test.md")
	os.WriteFile(mdPath, []byte("---\nname: md-test\n---\nBody"), 0644)

	skill, err := LoadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Format != FormatYAML {
		t.Errorf("expected yaml, got %s", skill.Format)
	}

	skill, err = LoadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Format != FormatMarkdown {
		t.Errorf("expected markdown, got %s", skill.Format)
	}
}

func TestLoadFileUnsupported(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "test.txt")
	os.WriteFile(txtPath, []byte("not a skill"), 0644)

	_, err := LoadFile(txtPath)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestLoadFromYAMLBackCompat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "old.yaml"), []byte("name: old-skill\ntrigger: /old\n"), 0644)

	skill, err := LoadFromYAML(filepath.Join(dir, "old.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "old-skill" {
		t.Errorf("expected old-skill, got %s", skill.Name)
	}
	if skill.Format != FormatYAML {
		t.Errorf("expected yaml format, got %s", skill.Format)
	}
}

func TestLoadFromDirLegacyBackCompat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("name: a\ntrigger: /a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("name: b\ntrigger: /b\n"), 0644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("not a skill"), 0644)

	skills, err := LoadFromDirLegacy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestLoadFromDirLegacyMissing(t *testing.T) {
	skills, err := LoadFromDirLegacy("/nonexistent")
	if err != nil {
		t.Fatal("expected nil error for missing dir")
	}
	if skills != nil {
		t.Error("expected nil skills")
	}
}
