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
