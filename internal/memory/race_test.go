package memory

import (
	"context"
	"sync"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestCreateWingIfNotExists_RaceCondition(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	wingName := "test-wing"

	var wg sync.WaitGroup
	results := make(chan string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := s.CreateWingIfNotExists(ctx, wingName)
			if err != nil {
				t.Errorf("CreateWingIfNotExists failed: %v", err)
				return
			}
			results <- id
		}()
	}

	wg.Wait()
	close(results)

	var firstID string
	duplicateFound := false
	for id := range results {
		if firstID == "" {
			firstID = id
		} else if id != firstID {
			duplicateFound = true
			break
		}
	}

	if duplicateFound {
		t.Error("CreateWingIfNotExists created duplicate wings under race condition")
	}

	wings, err := s.GetWings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(wings) != 1 {
		t.Errorf("expected exactly 1 wing, got %d", len(wings))
	}
}

func TestCreateRoomIfNotExists_RaceCondition(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "test-wing"}
	if err := s.CreateWing(ctx, wing); err != nil {
		t.Fatal(err)
	}

	roomName := "test-room"
	var wg sync.WaitGroup
	results := make(chan string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := s.CreateRoomIfNotExists(ctx, wing.ID, roomName, "facts")
			if err != nil {
				t.Errorf("CreateRoomIfNotExists failed: %v", err)
				return
			}
			results <- id
		}()
	}

	wg.Wait()
	close(results)

	var firstID string
	duplicateFound := false
	for id := range results {
		if firstID == "" {
			firstID = id
		} else if id != firstID {
			duplicateFound = true
			break
		}
	}

	if duplicateFound {
		t.Error("CreateRoomIfNotExists created duplicate rooms under race condition")
	}

	rooms, err := s.GetRooms(ctx, wing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected exactly 1 room, got %d", len(rooms))
	}
}

func TestStore_RollbackOnIndexFailure(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "test-wing"}
	if err := s.CreateWing(ctx, wing); err != nil {
		t.Fatal(err)
	}
	room := &cobot.Room{WingID: wing.ID, Name: "test-room", HallType: "facts"}
	if err := s.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	_, err = s.Store(ctx, "test content", wing.ID, room.ID)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	drawers, err := s.searchDrawers(ctx, &cobot.SearchQuery{Text: "test content"})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, d := range drawers {
		if d.Content == "test content" {
			found = true
			break
		}
	}

	if !found {
		t.Error("stored content should be searchable after successful Store")
	}
}

func TestDeleteDrawer(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "test-wing"}
	if err := s.CreateWing(ctx, wing); err != nil {
		t.Fatal(err)
	}
	room := &cobot.Room{WingID: wing.ID, Name: "test-room", HallType: "facts"}
	if err := s.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	id, err := s.AddDrawer(ctx, wing.ID, room.ID, "content to delete")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.GetDrawer(ctx, id)
	if err != nil {
		t.Fatalf("should be able to get drawer before deletion: %v", err)
	}

	if err := s.DeleteDrawer(ctx, id); err != nil {
		t.Fatalf("DeleteDrawer failed: %v", err)
	}

	_, err = s.GetDrawer(ctx, id)
	if err == nil {
		t.Error("should not be able to get drawer after deletion")
	}
}
