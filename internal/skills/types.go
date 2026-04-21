package skills

// Skill represents a loaded skill with full metadata.
type Skill struct {
	Name        string            // from frontmatter
	Description string            // from frontmatter
	Category    string            // parent directory name (empty if no category)
	Content     string            // markdown body (after frontmatter)
	Source      string            // "global" or "workspace"
	Dir         string            // absolute path to skill directory
	Metadata    map[string]string // optional frontmatter metadata
}

// SkillSummary is the tier-1 catalog view.
type SkillSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
	Source      string `json:"source,omitempty"`
}
