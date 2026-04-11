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
	if skill.Name != "myskill.yaml" {
		t.Errorf("expected myskill.yaml, got %s", skill.Name)
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
