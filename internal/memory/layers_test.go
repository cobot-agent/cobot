package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestWakeUpL0(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	result, err := s.WakeUpToLayer(ctx, L0Identity)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "You are Cobot") {
		t.Errorf("expected identity prompt, got: %s", result)
	}
}

func TestWakeUpL1(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	wing := &cobot.Wing{Name: "test"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "facts", HallType: "facts"}
	s.CreateRoom(ctx, room)
	closet := &cobot.Closet{RoomID: room.ID, Summary: "Important fact about testing"}
	s.CreateCloset(ctx, closet)

	result, err := s.WakeUpToLayer(ctx, L1Facts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Known Facts") {
		t.Errorf("expected facts section, got: %s", result)
	}
	if !strings.Contains(result, "Important fact about testing") {
		t.Errorf("expected fact content, got: %s", result)
	}
}

func TestWakeUpL2(t *testing.T) {
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

	_, err = s.Store(ctx, "First note about the project", wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	result, err := s.WakeUpToLayer(ctx, L2RoomRecall)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Room Context") {
		t.Errorf("expected room context section, got: %s", result)
	}
	if !strings.Contains(result, "myproject") {
		t.Errorf("expected wing name, got: %s", result)
	}
	if !strings.Contains(result, "notes") {
		t.Errorf("expected room name, got: %s", result)
	}
}

func TestWakeUpDefaultsToL2(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	result, err := s.WakeUp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "You are Cobot") {
		t.Errorf("expected identity, got: %s", result)
	}
}

func TestWakeUpIgnoresNonFactsAtL1(t *testing.T) {
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

	closet := &cobot.Closet{
		RoomID:  room.ID,
		Summary: "this should be ignored at L1",
	}
	s.CreateCloset(ctx, closet)

	got, err := s.WakeUpToLayer(ctx, L1Facts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "this should be ignored") {
		t.Errorf("L1 should ignore non-fact rooms, got %q", got)
	}
}
