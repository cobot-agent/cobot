package memory

import (
	"context"
	"testing"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestOpenCloseStore(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWingCRUD(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "test-project", Type: "project", Keywords: []string{"go", "agent"}}
	if err := s.CreateWing(ctx, wing); err != nil {
		t.Fatal(err)
	}
	if wing.ID == "" {
		t.Error("expected wing ID to be set")
	}

	got, err := s.GetWing(ctx, wing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-project" {
		t.Errorf("expected test-project, got %s", got.Name)
	}

	wings, err := s.GetWings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(wings) != 1 {
		t.Errorf("expected 1 wing, got %d", len(wings))
	}
}

func TestRoomCRUD(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)

	room := &cobot.Room{WingID: wing.ID, Name: "auth-migration", HallType: "facts"}
	if err := s.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	if room.ID == "" {
		t.Error("expected room ID")
	}

	rooms, err := s.GetRooms(ctx, wing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(rooms))
	}
	if rooms[0].Name != "auth-migration" {
		t.Errorf("expected auth-migration, got %s", rooms[0].Name)
	}
}

func TestDrawerCRUD(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "decisions", HallType: "facts"}
	s.CreateRoom(ctx, room)

	id, err := s.AddDrawer(ctx, wing.ID, room.ID, "decided to use BadgerDB for storage")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("expected drawer ID")
	}

	drawer, err := s.GetDrawer(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if drawer.Content != "decided to use BadgerDB for storage" {
		t.Errorf("unexpected content: %s", drawer.Content)
	}
	if drawer.RoomID != room.ID {
		t.Errorf("expected room ID %s, got %s", room.ID, drawer.RoomID)
	}
}

func TestClosetCRUD(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "summary", HallType: "facts"}
	s.CreateRoom(ctx, room)

	drawerID, _ := s.AddDrawer(ctx, wing.ID, room.ID, "content here")

	closet := &cobot.Closet{
		RoomID:    room.ID,
		DrawerIDs: []string{drawerID},
		Summary:   "brief summary",
	}
	if err := s.CreateCloset(ctx, closet); err != nil {
		t.Fatal(err)
	}

	closets, err := s.GetClosets(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(closets) != 1 {
		t.Fatalf("expected 1 closet, got %d", len(closets))
	}
	if closets[0].Summary != "brief summary" {
		t.Errorf("unexpected summary: %s", closets[0].Summary)
	}
}

func TestBleveSearch(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "decisions", HallType: "facts"}
	s.CreateRoom(ctx, room)

	id, _ := s.AddDrawer(ctx, wing.ID, room.ID, "decided to use BadgerDB for storage")
	s.indexDrawer(ctx, &drawerDoc{
		ID:       id,
		Content:  "decided to use BadgerDB for storage",
		WingID:   wing.ID,
		RoomID:   room.ID,
		HallType: "facts",
	})

	results, err := s.searchDrawers(ctx, &cobot.SearchQuery{Text: "BadgerDB"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected search results for 'BadgerDB'")
	}
	if results[0].Content != "decided to use BadgerDB for storage" {
		t.Errorf("unexpected result: %s", results[0].Content)
	}
}

func TestStoreAndSearch(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "decisions", HallType: "facts"}
	s.CreateRoom(ctx, room)

	_, err := s.Store(ctx, "decided to use BadgerDB for storage", wing.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	results, err := s.Search(ctx, &cobot.SearchQuery{Text: "BadgerDB"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'BadgerDB'")
	}
	found := false
	for _, r := range results {
		if r.Content == "decided to use BadgerDB for storage" {
			found = true
			break
		}
	}
	if !found {
		t.Error("stored content not found in search results")
	}
}

func TestStoreAndSearchMultiple(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	defer s.Close()

	ctx := context.Background()
	wing := &cobot.Wing{Name: "proj", Type: "project"}
	s.CreateWing(ctx, wing)
	room := &cobot.Room{WingID: wing.ID, Name: "notes", HallType: "facts"}
	s.CreateRoom(ctx, room)

	s.Store(ctx, "decided to use BadgerDB for storage", wing.ID, room.ID)
	s.Store(ctx, "the API gateway handles routing", wing.ID, room.ID)
	s.Store(ctx, "BadgerDB compaction runs on schedule", wing.ID, room.ID)

	results, err := s.Search(ctx, &cobot.SearchQuery{Text: "BadgerDB"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Content == "the API gateway handles routing" {
			t.Error("non-matching content should not appear in results")
		}
	}
	if results[0].Score < results[1].Score {
		t.Error("expected results sorted by descending score")
	}
}
