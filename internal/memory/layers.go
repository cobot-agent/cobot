package memory

import (
	"context"
	"log/slog"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type MemoryLayer int

const (
	L0Identity MemoryLayer = iota
	L1Facts
	L2RoomRecall
	L3DeepSearch
)

func (s *Store) WakeUp(ctx context.Context) (string, error) {
	return s.WakeUpToLayer(ctx, L2RoomRecall)
}

func (s *Store) WakeUpToLayer(ctx context.Context, layer MemoryLayer) (string, error) {
	l0 := "You are Cobot, a personal AI assistant."

	if layer == L0Identity {
		return l0, nil
	}

	var sections []string
	sections = append(sections, l0)

	wings, err := s.GetWings(ctx)
	if err != nil {
		return "", err
	}

	if layer >= L1Facts {
		facts := s.collectL1Facts(ctx, wings)
		if len(facts) > 0 {
			sections = append(sections, "## Known Facts")
			sections = append(sections, facts...)
		}
	}

	if layer >= L2RoomRecall {
		roomContexts := s.collectL2RoomRecall(ctx, wings)
		if len(roomContexts) > 0 {
			sections = append(sections, "## Room Context")
			sections = append(sections, roomContexts...)
		}
	}

	if layer >= L3DeepSearch {
		deepResults := s.collectL3DeepSearch(ctx, wings, "")
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

func (s *Store) collectL1Facts(ctx context.Context, wings []*cobot.Wing) []string {
	var facts []string
	for _, w := range wings {
		rooms, err := s.GetRooms(ctx, w.ID)
		if err != nil {
			slog.Warn("failed to get rooms", "wing", w.ID, "error", err)
			continue
		}
		for _, r := range rooms {
			if r.HallType != "facts" {
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

func (s *Store) collectL2RoomRecall(ctx context.Context, wings []*cobot.Wing) []string {
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
				RoomID: r.ID,
				Limit:  5,
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
