package bootstrap

import (
	"testing"
)

func TestReplaceSkillsSection(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		newSection string
		want       string
	}{
		{
			name:       "no existing section appends",
			current:    "system prompt",
			newSection: "## Skills (mandatory)\nskill content",
			want:       "system prompt\n\n## Skills (mandatory)\nskill content",
		},
		{
			name:       "replaces section at end",
			current:    "system prompt\n## Skills (mandatory)\nold skills",
			newSection: "## Skills (mandatory)\nnew skills",
			want:       "system prompt\n## Skills (mandatory)\nnew skills",
		},
		{
			name:       "replaces section in middle",
			current:    "system prompt\n## Skills (mandatory)\nold skills\n## Other\nmore",
			newSection: "## Skills (mandatory)\nnew skills",
			want:       "system prompt\n## Skills (mandatory)\nnew skills\n## Other\nmore",
		},
		{
			name:       "replaces section at start",
			current:    "## Skills (mandatory)\nold skills\n## Other\nmore",
			newSection: "## Skills (mandatory)\nnew skills",
			want:       "## Skills (mandatory)\nnew skills\n## Other\nmore",
		},
		{
			name:       "does not match inside code block",
			current:    "system\n```\n## Skills (mandatory)\nfake\n```\n## Other\nmore",
			newSection: "## Skills (mandatory)\nreal",
			want:       "system\n```\n## Skills (mandatory)\nfake\n```\n## Other\nmore\n\n## Skills (mandatory)\nreal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceSkillsSection(tt.current, tt.newSection)
			if got != tt.want {
				t.Errorf("replaceSkillsSection() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
