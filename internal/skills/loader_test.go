package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSkills_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	skills, err := LoadSkills(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkills_NonexistentDir(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope")
	skills, err := LoadSkills(context.Background(), []string{missing}, nil)
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkills_MarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "coding.md"), []byte("# Coding Expert\nYou are an expert coder."), 0644)
	os.WriteFile(filepath.Join(dir, "review.md"), []byte("## Code Review\nReview code carefully."), 0644)

	skills, err := LoadSkills(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	byName := make(map[string]Skill)
	for _, s := range skills {
		byName[s.Name] = s
	}

	if s, ok := byName["coding"]; !ok {
		t.Error("missing 'coding' skill")
	} else {
		if s.Source != "global" {
			t.Errorf("expected source 'global', got %q", s.Source)
		}
		if s.Description != "Coding Expert" {
			t.Errorf("expected description 'Coding Expert', got %q", s.Description)
		}
		if !strings.Contains(s.Content, "expert coder") {
			t.Errorf("unexpected content: %q", s.Content)
		}
	}

	if _, ok := byName["review"]; !ok {
		t.Error("missing 'review' skill")
	}
}

func TestLoadSkills_YAMLFiles(t *testing.T) {
	dir := t.TempDir()
	content := `
name: planner
description: Task planning
content: |
  You are a task planner.
  Break down goals into steps.
`
	os.WriteFile(filepath.Join(dir, "planner.yaml"), []byte(content), 0644)

	skills, err := LoadSkills(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Name != "planner" {
		t.Errorf("name = %q, want planner", s.Name)
	}
	if s.Description != "Task planning" {
		t.Errorf("description = %q", s.Description)
	}
	if !strings.Contains(s.Content, "task planner") {
		t.Errorf("unexpected content: %q", s.Content)
	}
}

func TestLoadSkills_YAMLWithYmlExt(t *testing.T) {
	dir := t.TempDir()
	content := `
name: tester
description: Test skill
content: Test content
`
	os.WriteFile(filepath.Join(dir, "tester.yml"), []byte(content), 0644)

	skills, err := LoadSkills(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "tester" {
		t.Errorf("name = %q, want tester", skills[0].Name)
	}
}

func TestLoadSkills_MergeOverride(t *testing.T) {
	globalDir := t.TempDir()
	workspaceDir := t.TempDir()

	os.WriteFile(filepath.Join(globalDir, "shared.md"), []byte("# Global Shared\nGlobal content"), 0644)
	os.WriteFile(filepath.Join(globalDir, "global-only.md"), []byte("# Only Global\nOnly in global"), 0644)
	os.WriteFile(filepath.Join(workspaceDir, "shared.md"), []byte("# Workspace Shared\nWorkspace content"), 0644)

	skills, err := LoadSkills(context.Background(), []string{globalDir, workspaceDir}, nil)
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d: %+v", len(skills), skills)
	}

	byName := make(map[string]Skill)
	for _, s := range skills {
		byName[s.Name] = s
	}

	s, ok := byName["shared"]
	if !ok {
		t.Fatal("missing 'shared' skill")
	}
	if s.Source != "workspace" {
		t.Errorf("shared skill source = %q, want workspace", s.Source)
	}
	if !strings.Contains(s.Content, "Workspace content") {
		t.Errorf("shared skill content should be from workspace: %q", s.Content)
	}

	if _, ok := byName["global-only"]; !ok {
		t.Error("missing 'global-only' skill")
	}
}

func TestLoadSkills_Filter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\nSkill A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\nSkill B"), 0644)
	os.WriteFile(filepath.Join(dir, "c.md"), []byte("# C\nSkill C"), 0644)

	skills, err := LoadSkills(context.Background(), []string{dir}, []string{"a", "c"})
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	byName := make(map[string]Skill)
	for _, s := range skills {
		byName[s.Name] = s
	}
	if _, ok := byName["a"]; !ok {
		t.Error("missing 'a'")
	}
	if _, ok := byName["b"]; ok {
		t.Error("'b' should be filtered out")
	}
	if _, ok := byName["c"]; !ok {
		t.Error("missing 'c'")
	}
}

func TestLoadSkills_FilterEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\nSkill A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\nSkill B"), 0644)

	// Empty filter means include all.
	skills, err := LoadSkills(context.Background(), []string{dir}, []string{})
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills with empty filter, got %d", len(skills))
	}
}

func TestLoadSkills_IgnoresOtherExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.md"), []byte("# Good\nGood skill"), 0644)
	os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("not a skill"), 0644)
	os.WriteFile(filepath.Join(dir, "also_bad.json"), []byte("{}"), 0644)

	skills, err := LoadSkills(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "good" {
		t.Errorf("name = %q, want good", skills[0].Name)
	}
}

func TestSkillsToPrompt(t *testing.T) {
	skills := []Skill{
		{
			Name:        "coding",
			Description: "Expert coder",
			Content:     "You are an expert coder.",
			Source:      "global",
		},
		{
			Name:        "review",
			Description: "Code reviewer",
			Content:     "Review code carefully.",
			Source:      "workspace",
		},
	}
	result := SkillsToPrompt(skills)

	if !strings.Contains(result, "## Skills") {
		t.Error("missing '## Skills' header")
	}
	if !strings.Contains(result, "### coding (global)") {
		t.Error("missing coding heading")
	}
	if !strings.Contains(result, "### review (workspace)") {
		t.Error("missing review heading")
	}
	if !strings.Contains(result, "> Expert coder") {
		t.Error("missing coding description")
	}
	if !strings.Contains(result, "You are an expert coder.") {
		t.Error("missing coding content")
	}
	if !strings.Contains(result, "Review code carefully.") {
		t.Error("missing review content")
	}
}

func TestSkillsToPrompt_Empty(t *testing.T) {
	result := SkillsToPrompt(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSkillsToPrompt_ContentNoTrailingNewline(t *testing.T) {
	skills := []Skill{
		{
			Name:    "test",
			Content: "no trailing newline", // no trailing \n
			Source:  "global",
		},
	}
	result := SkillsToPrompt(skills)
	if !strings.Contains(result, "no trailing newline\n") {
		t.Error("should have appended newline after content")
	}
}
