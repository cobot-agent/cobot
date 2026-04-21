package skills

import (
	"testing"
)

func TestParseFrontMatter(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantName  string
		wantDesc  string
		wantBody  string
		wantErr   bool
		checkMeta func(*testing.T, map[string]string)
	}{
		{
			"valid with metadata",
			"---\nname: code-review\ndescription: Review code for quality and security.\nmetadata:\n  author: cobot\n  version: \"1.0\"\n---\n\n# Code Review\n\n## Steps\n1. Read the diff\n",
			"code-review", "Review code for quality and security.",
			"\n# Code Review\n\n## Steps\n1. Read the diff",
			false,
			func(t *testing.T, m map[string]string) {
				if m == nil || m["author"] != "cobot" {
					t.Errorf("metadata.author = %v", m)
				}
				if m == nil || m["version"] != "1.0" {
					t.Errorf("metadata.version = %v", m)
				}
			},
		},
		{
			"no metadata",
			"---\nname: simple\ndescription: A simple skill\n---\n\nSimple body.\n",
			"simple", "A simple skill", "\nSimple body.", false,
			func(t *testing.T, m map[string]string) {
				if m != nil {
					t.Errorf("expected nil metadata, got %v", m)
				}
			},
		},
		{"no frontmatter", "Just regular markdown", "", "", "", true, nil},
		{"missing close delimiter", "---\nname: broken\ndescription: Missing close\n", "", "", "", true, nil},
		{"empty yaml", "---\n---\nBody content.", "", "", "Body content.", false, nil},
		{
			"crlf",
			"---\r\nname: crlf-skill\r\ndescription: Skill with CRLF\r\n---\r\nBody here.\r\n",
			"crlf-skill", "Skill with CRLF", "Body here.", false, nil,
		},
		{
			"dashes in value",
			"---\nname: test-skill\ndescription: \"Use --- for separators\"\n---\nBody text.",
			"test-skill", "Use --- for separators", "Body text.", false, nil,
		},
		{
			"no body",
			"---\nname: no-body\ndescription: Skill with no body\n---\n",
			"no-body", "Skill with no body", "", false, nil,
		},
		{"empty yaml trailing newline", "---\n---\n", "", "", "", false, nil},
		{"empty yaml no trailing newline", "---\n---", "", "", "", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := parseFrontMatter(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if fm.Name != tt.wantName {
				t.Errorf("name = %q, want %q", fm.Name, tt.wantName)
			}
			if fm.Description != tt.wantDesc {
				t.Errorf("description = %q, want %q", fm.Description, tt.wantDesc)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if tt.checkMeta != nil {
				tt.checkMeta(t, fm.Metadata)
			}
		})
	}
}
