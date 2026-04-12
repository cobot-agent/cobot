package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestL3DeepSearch(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	wing := &cobot.Wing{Name: "test"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "notes", HallType: "log"}
	s.CreateRoom(ctx, room)

	_, err = s.Store(ctx, "Important decision about architecture", wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Store(ctx, "Another document about implementation", wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	results, err := s.L3DeepSearch(ctx, "decision", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Error("expected search results")
	}

	found := false
	for _, r := range results {
		if strings.Contains(r.Content, "Important decision") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'Important decision' in results")
	}
}

func TestSummarizeContent(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "This is a long line that should be summarized properly\nMore content here",
			expected: "This is a long line that should be summarized properly",
		},
		{
			input:    "Short",
			expected: "Short",
		},
		{
			input:    strings.Repeat("a", 300),
			expected: strings.Repeat("a", 200) + "...",
		},
	}

	for _, tt := range tests {
		result := s.SummarizeContent(tt.input)
		if result != tt.expected {
			t.Errorf("SummarizeContent(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestAutoSummarizeRoom(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	wing := &cobot.Wing{Name: "test"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "notes", HallType: "log"}
	s.CreateRoom(ctx, room)

	_, err = s.Store(ctx, "First important note about the project", wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Store(ctx, "Second note with more details", wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	err = s.AutoSummarizeRoom(ctx, wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	closets, err := s.GetClosets(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(closets) == 0 {
		t.Error("expected at least one closet after auto-summarize")
	}

	found := false
	for _, c := range closets {
		if c.Summary != "" && (strings.Contains(c.Summary, "First important note") || strings.Contains(c.Summary, "Second note")) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected closet with summary containing note content")
	}
}

func TestWakeUpL3(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	wing := &cobot.Wing{Name: "myproject"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "notes", HallType: "log"}
	s.CreateRoom(ctx, room)

	_, err = s.Store(ctx, "Important decision made about the architecture", wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	result, err := s.WakeUpToLayer(ctx, L3DeepSearch)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Deep Search") {
		t.Errorf("expected deep search section, got: %s", result)
	}

	if !strings.Contains(result, "Important decision") && !strings.Contains(result, "Related:") {
		t.Errorf("expected deep search content, got: %s", result)
	}
}
