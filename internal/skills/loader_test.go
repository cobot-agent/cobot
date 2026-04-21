package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalog_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	skills, err := LoadCatalog(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadCatalog_NonexistentDir(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope")
	skills, err := LoadCatalog(context.Background(), []string{missing}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadCatalog_NewFormatSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "code-review")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: code-review
description: Review code for quality and security.
metadata:
  author: cobot
  version: "1.0"
---

# Code Review

## Steps
1. Read the diff
`), 0644)

	skills, err := LoadCatalog(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Name != "code-review" {
		t.Errorf("name = %q, want code-review", s.Name)
	}
	if s.Description != "Review code for quality and security." {
		t.Errorf("description = %q", s.Description)
	}
	if s.Source != "global" {
		t.Errorf("source = %q, want global", s.Source)
	}
	if s.Category != "" {
		t.Errorf("category = %q, want empty", s.Category)
	}
	if s.Metadata == nil || s.Metadata["author"] != "cobot" {
		t.Errorf("metadata = %v", s.Metadata)
	}
	if !strings.Contains(s.Content, "# Code Review") {
		t.Errorf("content missing body: %q", s.Content)
	}
}

func TestLoadCatalog_NewFormatWithCategory(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "coding", "code-review")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: code-review
description: Review code.
---

# Code Review
`), 0644)

	skills, err := LoadCatalog(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Category != "coding" {
		t.Errorf("category = %q, want coding", skills[0].Category)
	}
}

func TestLoadCatalog_MarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "coding.md"), []byte("# Coding Expert\nYou are an expert coder."), 0644)
	os.WriteFile(filepath.Join(dir, "review.md"), []byte("## Code Review\nReview code carefully."), 0644)

	skills, err := LoadCatalog(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
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

func TestLoadCatalog_YAMLFiles(t *testing.T) {
	dir := t.TempDir()
	content := `
name: planner
description: Task planning
content: |
  You are a task planner.
  Break down goals into steps.
`
	os.WriteFile(filepath.Join(dir, "planner.yaml"), []byte(content), 0644)

	skills, err := LoadCatalog(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
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

func TestLoadCatalog_YAMLWithYmlExt(t *testing.T) {
	dir := t.TempDir()
	content := `
name: tester
description: Test skill
content: Test content
`
	os.WriteFile(filepath.Join(dir, "tester.yml"), []byte(content), 0644)

	skills, err := LoadCatalog(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "tester" {
		t.Errorf("name = %q, want tester", skills[0].Name)
	}
}

func TestLoadCatalog_MergeOverride(t *testing.T) {
	globalDir := t.TempDir()
	workspaceDir := t.TempDir()

	os.WriteFile(filepath.Join(globalDir, "shared.md"), []byte("# Global Shared\nGlobal content"), 0644)
	os.WriteFile(filepath.Join(globalDir, "global-only.md"), []byte("# Only Global\nOnly in global"), 0644)
	os.WriteFile(filepath.Join(workspaceDir, "shared.md"), []byte("# Workspace Shared\nWorkspace content"), 0644)

	skills, err := LoadCatalog(context.Background(), []string{globalDir, workspaceDir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
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

func TestLoadCatalog_Filter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\nSkill A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\nSkill B"), 0644)
	os.WriteFile(filepath.Join(dir, "c.md"), []byte("# C\nSkill C"), 0644)

	skills, err := LoadCatalog(context.Background(), []string{dir}, []string{"a", "c"})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
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

func TestLoadCatalog_FilterEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\nSkill A"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\nSkill B"), 0644)

	// Empty filter means include all.
	skills, err := LoadCatalog(context.Background(), []string{dir}, []string{})
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills with empty filter, got %d", len(skills))
	}
}

func TestLoadCatalog_IgnoresOtherExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.md"), []byte("# Good\nGood skill"), 0644)
	os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("not a skill"), 0644)
	os.WriteFile(filepath.Join(dir, "also_bad.json"), []byte("{}"), 0644)

	skills, err := LoadCatalog(context.Background(), []string{dir}, nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "good" {
		t.Errorf("name = %q, want good", skills[0].Name)
	}
}

func TestLoadFull_Found(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: my-skill
description: A test skill
---

Body content here.
`), 0644)

	sk, err := LoadFull(context.Background(), []string{dir}, "my-skill")
	if err != nil {
		t.Fatalf("LoadFull: %v", err)
	}
	if sk.Name != "my-skill" {
		t.Errorf("name = %q", sk.Name)
	}
	if !strings.Contains(sk.Content, "Body content here.") {
		t.Errorf("content = %q", sk.Content)
	}
}

func TestLoadFull_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadFull(context.Background(), []string{dir}, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestListLinkedFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "references"), 0755)
	os.MkdirAll(filepath.Join(dir, "templates"), 0755)
	os.WriteFile(filepath.Join(dir, "references", "api.md"), []byte("api docs"), 0644)
	os.WriteFile(filepath.Join(dir, "references", "guide.md"), []byte("guide"), 0644)
	os.WriteFile(filepath.Join(dir, "templates", "output.md"), []byte("template"), 0644)

	files := ListLinkedFiles(dir)
	if len(files["references"]) != 2 {
		t.Errorf("references = %v, want 2 files", files["references"])
	}
	if len(files["templates"]) != 1 {
		t.Errorf("templates = %v, want 1 file", files["templates"])
	}
	if _, ok := files["scripts"]; ok {
		t.Error("scripts should not be present")
	}
}

func TestReadLinkedFile_Valid(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "references"), 0755)
	os.WriteFile(filepath.Join(dir, "references", "api.md"), []byte("api docs content"), 0644)

	content, err := ReadLinkedFile(dir, "references/api.md")
	if err != nil {
		t.Fatalf("ReadLinkedFile: %v", err)
	}
	if content != "api docs content" {
		t.Errorf("content = %q", content)
	}
}

func TestReadLinkedFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadLinkedFile(dir, "../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestReadLinkedFile_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadLinkedFile(dir, "/etc/passwd")
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestReadLinkedFile_InvalidSubdir(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadLinkedFile(dir, "other/file.txt")
	if err == nil {
		t.Error("expected error for invalid subdir")
	}
}

func TestSkillsToPrompt_Tier1(t *testing.T) {
	skills := []Skill{
		{
			Name:        "coding",
			Description: "Expert coder",
			Source:      "global",
		},
		{
			Name:        "review",
			Description: "Code reviewer",
			Category:    "coding",
			Source:      "workspace",
		},
	}
	result := SkillsToPrompt(skills)

	if !strings.Contains(result, "## Skills") {
		t.Error("missing '## Skills' header")
	}
	if !strings.Contains(result, "**coding**: Expert coder") {
		t.Error("missing coding summary")
	}
	if !strings.Contains(result, "**review** (coding): Code reviewer") {
		t.Error("missing review with category")
	}
	// Tier-1 should NOT include full content
	if strings.Contains(result, "You are") {
		t.Error("tier-1 prompt should not include full content")
	}
}

func TestSkillsToPrompt_Empty(t *testing.T) {
	result := SkillsToPrompt(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFindSkillDir_NewFormat(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\ndescription: test\n---\nbody"), 0644)

	found, err := FindSkillDir(dir, t.TempDir(), "my-skill")
	if err != nil {
		t.Fatalf("FindSkillDir: %v", err)
	}
	if found != skillDir {
		t.Errorf("found = %q, want %q", found, skillDir)
	}
}

func TestFindSkillDir_WithCategory(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "coding", "review")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: review\ndescription: test\n---\nbody"), 0644)

	found, err := FindSkillDir(dir, t.TempDir(), "review")
	if err != nil {
		t.Fatalf("FindSkillDir: %v", err)
	}
	if found != skillDir {
		t.Errorf("found = %q, want %q", found, skillDir)
	}
}

func TestFindSkillDir_LegacyMd(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "old-skill.md"), []byte("# Old Skill\nContent"), 0644)

	found, err := FindSkillDir(dir, t.TempDir(), "old-skill")
	if err != nil {
		t.Fatalf("FindSkillDir: %v", err)
	}
	if found != dir {
		t.Errorf("found = %q, want %q", found, dir)
	}
}

func TestFindSkillDir_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindSkillDir(dir, t.TempDir(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}
