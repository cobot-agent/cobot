package skills

import (
	"testing"
)

func TestParseFrontMatter_Valid(t *testing.T) {
	content := `---
name: code-review
description: Review code for quality and security.
metadata:
  author: cobot
  version: "1.0"
---

# Code Review

## Steps
1. Read the diff
`

	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter: %v", err)
	}
	if fm.Name != "code-review" {
		t.Errorf("name = %q, want code-review", fm.Name)
	}
	if fm.Description != "Review code for quality and security." {
		t.Errorf("description = %q", fm.Description)
	}
	if fm.Metadata == nil {
		t.Fatal("metadata is nil")
	}
	if fm.Metadata["author"] != "cobot" {
		t.Errorf("metadata.author = %q", fm.Metadata["author"])
	}
	if fm.Metadata["version"] != "1.0" {
		t.Errorf("metadata.version = %q", fm.Metadata["version"])
	}
	if body != "\n# Code Review\n\n## Steps\n1. Read the diff" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontMatter_NoMetadata(t *testing.T) {
	content := `---
name: simple
description: A simple skill
---

Simple body.
`
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter: %v", err)
	}
	if fm.Name != "simple" {
		t.Errorf("name = %q, want simple", fm.Name)
	}
	if fm.Metadata != nil {
		t.Errorf("expected nil metadata, got %v", fm.Metadata)
	}
	if body != "\nSimple body." {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontMatter_NoFrontmatter(t *testing.T) {
	content := "Just regular markdown"
	_, _, err := ParseFrontMatter(content)
	if err == nil {
		t.Error("expected error for content without frontmatter")
	}
}

func TestParseFrontMatter_MissingCloseDelimiter(t *testing.T) {
	content := `---
name: broken
description: Missing close
`
	_, _, err := ParseFrontMatter(content)
	if err == nil {
		t.Error("expected error for missing closing delimiter")
	}
}

func TestParseFrontMatter_EmptyYAML(t *testing.T) {
	content := "---\n---\nBody content."
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter with empty YAML: %v", err)
	}
	if fm.Name != "" {
		t.Errorf("name = %q, want empty", fm.Name)
	}
	if body != "Body content." {
		t.Errorf("body = %q, want %q", body, "Body content.")
	}
}

func TestParseFrontMatter_CRLF(t *testing.T) {
	content := "---\r\nname: crlf-skill\r\ndescription: Skill with CRLF\r\n---\r\nBody here.\r\n"
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter with CRLF: %v", err)
	}
	if fm.Name != "crlf-skill" {
		t.Errorf("name = %q, want crlf-skill", fm.Name)
	}
	if fm.Description != "Skill with CRLF" {
		t.Errorf("description = %q", fm.Description)
	}
	if body != "Body here." {
		t.Errorf("body = %q, want %q", body, "Body here.")
	}
}

func TestParseFrontMatter_DashesInValue(t *testing.T) {
	// Ensure that a value containing "---" doesn't break the parser.
	content := "---\nname: test-skill\ndescription: \"Use --- for separators\"\n---\nBody text."
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter with dashes in value: %v", err)
	}
	if fm.Name != "test-skill" {
		t.Errorf("name = %q, want test-skill", fm.Name)
	}
	if fm.Description != "Use --- for separators" {
		t.Errorf("description = %q", fm.Description)
	}
	if body != "Body text." {
		t.Errorf("body = %q, want %q", body, "Body text.")
	}
}

func TestParseFrontMatter_NoBody(t *testing.T) {
	content := "---\nname: no-body\ndescription: Skill with no body\n---\n"
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter with no body: %v", err)
	}
	if fm.Name != "no-body" {
		t.Errorf("name = %q, want no-body", fm.Name)
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestParseFrontMatter_EmptyYAMLTrailingNewline(t *testing.T) {
	// "---\n---\n" → after TrimSpace becomes "---\n---" which should still parse.
	content := "---\n---\n"
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter with empty YAML and trailing newline: %v", err)
	}
	if fm.Name != "" {
		t.Errorf("name = %q, want empty", fm.Name)
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestParseFrontMatter_EmptyYAMLNoTrailingNewline(t *testing.T) {
	content := "---\n---"
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter with empty YAML no trailing newline: %v", err)
	}
	if fm.Name != "" {
		t.Errorf("name = %q, want empty", fm.Name)
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}
