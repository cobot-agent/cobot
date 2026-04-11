package memory

import (
	"context"
	"strings"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestWakeUpEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	got, err := s.WakeUp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expected := "You are Cobot, a personal AI assistant."
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestWakeUpWithFacts(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "facts-room", HallType: "facts"}
	s.CreateRoom(ctx, room)

	drawerID, _ := s.AddDrawer(ctx, wing.ID, room.ID, "some content")
	closet := &cobot.Closet{
		RoomID:    room.ID,
		DrawerIDs: []string{drawerID},
		Summary:   "user prefers dark mode",
	}
	s.CreateCloset(ctx, closet)

	got, err := s.WakeUp(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(got, "You are Cobot, a personal AI assistant.") {
		t.Errorf("expected identity prefix, got %q", got)
	}
	if !strings.Contains(got, "user prefers dark mode") {
		t.Errorf("expected summary in output, got %q", got)
	}
}

func TestWakeUpIgnoresNonFacts(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "log-room", HallType: "log"}
	s.CreateRoom(ctx, room)

	drawerID, _ := s.AddDrawer(ctx, wing.ID, room.ID, "log entry")
	closet := &cobot.Closet{
		RoomID:    room.ID,
		DrawerIDs: []string{drawerID},
		Summary:   "this should be ignored",
	}
	s.CreateCloset(ctx, closet)

	got, err := s.WakeUp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expected := "You are Cobot, a personal AI assistant."
	if got != expected {
		t.Errorf("expected identity only, got %q", got)
	}
}
