package memory

import (
	"context"
	"strings"
)

func (s *Store) WakeUp(ctx context.Context) (string, error) {
	l0 := "You are Cobot, a personal AI assistant."

	var summaries []string
	wings, err := s.GetWings(ctx)
	if err != nil {
		return "", err
	}
	for _, w := range wings {
		rooms, err := s.GetRooms(ctx, w.ID)
		if err != nil {
			return "", err
		}
		for _, r := range rooms {
			if r.HallType != "facts" {
				continue
			}
			closets, err := s.GetClosets(ctx, r.ID)
			if err != nil {
				return "", err
			}
			for _, c := range closets {
				if c.Summary != "" {
					summaries = append(summaries, c.Summary)
				}
			}
		}
	}

	if len(summaries) == 0 {
		return l0, nil
	}

	var b strings.Builder
	b.WriteString(l0)
	for _, s := range summaries {
		b.WriteString("\n\n")
		b.WriteString(s)
	}
	return b.String(), nil
}
