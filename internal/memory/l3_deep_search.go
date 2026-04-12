package memory

import (
	"context"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// L3DeepSearch performs comprehensive semantic search across all memory.
// It searches for relevant content based on the query and recent context.
func (s *Store) L3DeepSearch(ctx context.Context, query string, limit int) ([]*cobot.SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	searchQuery := &cobot.SearchQuery{
		Text:  query,
		Limit: limit,
	}

	return s.searchDrawers(ctx, searchQuery)
}

// collectL3DeepSearch performs context-aware deep search for WakeUp L3 layer.
// It searches for content relevant to recent conversation context.
func (s *Store) collectL3DeepSearch(ctx context.Context, wings []*cobot.Wing, contextHint string) []string {
	var results []string

	queries := s.generateL3Queries(contextHint)

	for _, query := range queries {
		searchResults, err := s.L3DeepSearch(ctx, query, 3)
		if err != nil || len(searchResults) == 0 {
			continue
		}

		var b strings.Builder
		b.WriteString("### Related: ")
		b.WriteString(query)
		for _, r := range searchResults {
			content := r.Content
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			b.WriteString("\n- [")
			b.WriteString(r.WingID)
			b.WriteString("] ")
			b.WriteString(content)
		}
		results = append(results, b.String())
	}

	return results
}

// generateL3Queries creates search queries based on context hint.
// If no hint provided, uses default exploratory queries.
func (s *Store) generateL3Queries(contextHint string) []string {
	if contextHint != "" {
		return []string{
			contextHint,
			contextHint + " recent",
		}
	}

	return []string{
		"important decision",
		"key insight",
		"lesson learned",
		"TODO",
		"important",
	}
}

// SummarizeContent generates a summary for a drawer using simple extraction.
// This is a lightweight summarization - more sophisticated NLP could be added.
func (s *Store) SummarizeContent(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 10 {
			if len(line) > 200 {
				return line[:200] + "..."
			}
			return line
		}
	}

	if len(content) > 200 {
		return content[:200] + "..."
	}
	return content
}

// AutoSummarizeRoom generates summaries for all closets in a room.
// Creates or updates closets with auto-generated summaries from recent drawers.
func (s *Store) AutoSummarizeRoom(ctx context.Context, wingID, roomID string) error {
	results, err := s.searchDrawers(ctx, &cobot.SearchQuery{
		RoomID: roomID,
		Limit:  10,
	})
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return nil
	}

	var summaries []string
	for _, r := range results {
		summary := s.SummarizeContent(r.Content)
		if summary != "" {
			summaries = append(summaries, summary)
		}
	}

	if len(summaries) == 0 {
		return nil
	}

	combinedSummary := strings.Join(summaries, "; ")
	if len(combinedSummary) > 500 {
		combinedSummary = combinedSummary[:500] + "..."
	}

	closet := &cobot.Closet{
		RoomID:  roomID,
		Summary: combinedSummary,
	}

	return s.CreateCloset(ctx, closet)
}
