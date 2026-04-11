package memory

import (
	"context"
	"testing"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func TestAddTripleQuery(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	triple := &cobot.Triple{
		Subject:   "alice",
		Predicate: "works_at",
		Object:    "acme",
	}
	if err := s.AddTriple(ctx, triple); err != nil {
		t.Fatal(err)
	}

	results, err := s.Query(ctx, "alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Predicate != "works_at" || results[0].Object != "acme" {
		t.Errorf("unexpected triple: %+v", results[0])
	}
}

func TestQueryWithTimeFilter(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	past := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	triple := &cobot.Triple{
		Subject:   "bob",
		Predicate: "lives_in",
		Object:    "london",
		ValidFrom: past,
	}
	if err := s.AddTriple(ctx, triple); err != nil {
		t.Fatal(err)
	}

	asOfIncluded := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	results, err := s.Query(ctx, "bob", &asOfIncluded)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for included time, got %d", len(results))
	}

	asOfExcluded := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	results, err = s.Query(ctx, "bob", &asOfExcluded)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for excluded time, got %d", len(results))
	}
}

func TestInvalidate(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	triple := &cobot.Triple{
		Subject:   "carol",
		Predicate: "role",
		Object:    "engineer",
	}
	if err := s.AddTriple(ctx, triple); err != nil {
		t.Fatal(err)
	}

	ended := time.Now().Add(-time.Second)
	if err := s.Invalidate(ctx, "carol", "role", "engineer", ended); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	results, err := s.Query(ctx, "carol", &now)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results after invalidation, got %d", len(results))
	}
}

func TestTimeline(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	triples := []*cobot.Triple{
		{Subject: "dave", Predicate: "lives_in", Object: "paris", ValidFrom: t3},
		{Subject: "dave", Predicate: "works_at", Object: "acme", ValidFrom: t1},
		{Subject: "dave", Predicate: "role", Object: "engineer", ValidFrom: t2},
	}
	for _, tr := range triples {
		if err := s.AddTriple(ctx, tr); err != nil {
			t.Fatal(err)
		}
	}

	results, err := s.Timeline(ctx, "dave")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !results[0].ValidFrom.Before(results[1].ValidFrom) || !results[1].ValidFrom.Before(results[2].ValidFrom) {
		t.Errorf("results not sorted by ValidFrom: %v, %v, %v", results[0].ValidFrom, results[1].ValidFrom, results[2].ValidFrom)
	}
	if results[0].Predicate != "works_at" {
		t.Errorf("expected first predicate works_at, got %s", results[0].Predicate)
	}
	if results[2].Predicate != "lives_in" {
		t.Errorf("expected last predicate lives_in, got %s", results[2].Predicate)
	}
}
