package memory

import (
	"context"
	"log/slog"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (s *Store) WakeUp(ctx context.Context) (string, error) {
	return s.WakeUpWithDeepSearch(ctx, false)
}

func (s *Store) WakeUpWithDeepSearch(ctx context.Context, deepSearch bool) (string, error) {
	identity := cobot.DefaultSystemPrompt

	wings, err := s.GetWings(ctx)
	if err != nil {
		return "", err
	}

	var sections []string
	sections = append(sections, identity)

	// Basic wakeup: collect facts and room context
	facts := s.collectFacts(ctx, wings)
	if len(facts) > 0 {
		sections = append(sections, "## Known Facts")
		sections = append(sections, facts...)
	}

	roomContexts := s.collectRoomRecall(ctx, wings)
	if len(roomContexts) > 0 {
		sections = append(sections, "## Room Context")
		sections = append(sections, roomContexts...)
	}

	// Deep search: semantic search across all memory
	if deepSearch {
		deepResults := s.collectDeepSearch(ctx, wings, "")
		if len(deepResults) > 0 {
			sections = append(sections, "## Deep Search")
			sections = append(sections, deepResults...)
		}
	}

	var b strings.Builder
	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(sec)
	}
	return b.String(), nil
}

func (s *Store) collectFacts(ctx context.Context, wings []*cobot.Wing) []string {
	var facts []string
	for _, w := range wings {
		rooms, err := s.GetRooms(ctx, w.ID)
		if err != nil {
			slog.Warn("failed to get rooms", "wing", w.ID, "error", err)
			continue
		}
		for _, r := range rooms {
			if r.HallType != cobot.TagFacts {
				continue
			}
			closets, err := s.GetClosets(ctx, r.ID)
			if err != nil {
				slog.Warn("failed to get closets", "room", r.ID, "error", err)
				continue
			}
			for _, c := range closets {
				if c.Summary != "" {
					facts = append(facts, "- "+c.Summary)
				}
			}
		}
	}
	return facts
}

func (s *Store) collectRoomRecall(ctx context.Context, wings []*cobot.Wing) []string {
	var contexts []string
	for _, w := range wings {
		rooms, err := s.GetRooms(ctx, w.ID)
		if err != nil {
			slog.Warn("failed to get rooms", "wing", w.ID, "error", err)
			continue
		}
		for _, r := range rooms {
			var b strings.Builder
			b.WriteString("### ")
			b.WriteString(w.Name)
			b.WriteString(" / ")
			b.WriteString(r.Name)
			b.WriteString(" (")
			b.WriteString(r.HallType)
			b.WriteString(")")

			drawers, err := s.searchDrawers(ctx, &cobot.SearchQuery{
				Tier2: r.ID,
				Limit: 5,
			})
			if err == nil && len(drawers) > 0 {
				for _, d := range drawers {
					content := d.Content
					if len(content) > 100 {
						content = content[:100] + "..."
					}
					b.WriteString("\n- ")
					b.WriteString(content)
				}
			}
			contexts = append(contexts, b.String())
		}
	}
	return contexts
}
