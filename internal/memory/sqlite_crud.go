package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// --- Wings ---

func (s *Store) CreateWing(ctx context.Context, wing *cobot.Wing) error {
	if wing.ID == "" {
		wing.ID = newID()
	}
	kw, err := json.Marshal(wing.Keywords)
	if err != nil {
		return fmt.Errorf("marshal keywords: %w", err)
	}
	_, err = s.db.ExecContext(ctx, sqlInsertWing, wing.ID, wing.Name, wing.Type, string(kw))
	return err
}

func (s *Store) GetWing(ctx context.Context, id string) (*cobot.Wing, error) {
	row := s.db.QueryRowContext(ctx, sqlSelectWing, id)
	return scanWing(row)
}

func (s *Store) GetWingByName(ctx context.Context, name string) (*cobot.Wing, error) {
	row := s.db.QueryRowContext(ctx, sqlSelectWingByName, name)
	w, err := scanWing(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return w, err
}

func (s *Store) GetWings(ctx context.Context) ([]*cobot.Wing, error) {
	rows, err := s.db.QueryContext(ctx, sqlSelectWings)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wings []*cobot.Wing
	for rows.Next() {
		w, err := scanWingRows(rows)
		if err != nil {
			return nil, err
		}
		wings = append(wings, w)
	}
	return wings, rows.Err()
}

func (s *Store) CreateWingIfNotExists(ctx context.Context, name string) (string, error) {
	existing, err := s.GetWingByName(ctx, name)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return existing.ID, nil
	}
	wing := &cobot.Wing{ID: newID(), Name: name}
	if err := s.CreateWing(ctx, wing); err != nil {
		return "", err
	}
	return wing.ID, nil
}

// --- Rooms ---

func (s *Store) CreateRoom(ctx context.Context, room *cobot.Room) error {
	if room.ID == "" {
		room.ID = newID()
	}
	_, err := s.db.ExecContext(ctx, sqlInsertRoom, room.ID, room.WingID, room.Name, room.HallType)
	return err
}

func (s *Store) GetRooms(ctx context.Context, wingID string) ([]*cobot.Room, error) {
	rows, err := s.db.QueryContext(ctx, sqlSelectRooms, wingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*cobot.Room
	for rows.Next() {
		var r cobot.Room
		if err := rows.Scan(&r.ID, &r.WingID, &r.Name, &r.HallType); err != nil {
			return nil, err
		}
		rooms = append(rooms, &r)
	}
	return rooms, rows.Err()
}

func (s *Store) GetRoom(ctx context.Context, wingID, roomID string) (*cobot.Room, error) {
	var r cobot.Room
	err := s.db.QueryRowContext(ctx, sqlSelectRoom, roomID, wingID).
		Scan(&r.ID, &r.WingID, &r.Name, &r.HallType)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) GetRoomByName(ctx context.Context, wingID, name string) (*cobot.Room, error) {
	var r cobot.Room
	err := s.db.QueryRowContext(ctx, sqlSelectRoomByName, wingID, name).
		Scan(&r.ID, &r.WingID, &r.Name, &r.HallType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) CreateRoomIfNotExists(ctx context.Context, wingID, name, hallType string) (string, error) {
	existing, err := s.GetRoomByName(ctx, wingID, name)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return existing.ID, nil
	}
	room := &cobot.Room{ID: newID(), WingID: wingID, Name: name, HallType: hallType}
	if err := s.CreateRoom(ctx, room); err != nil {
		return "", err
	}
	return room.ID, nil
}

// --- Drawers ---

func (s *Store) AddDrawer(ctx context.Context, wingID, roomID, content string) (string, error) {
	id := newID()
	_, err := s.db.ExecContext(ctx, sqlInsertDrawer, id, roomID, content, "", time.Now().UTC())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) DeleteDrawer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, sqlDeleteDrawer, id)
	return err
}

// --- Closets ---

func (s *Store) CreateCloset(ctx context.Context, closet *cobot.Closet) error {
	if closet.ID == "" {
		closet.ID = newID()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, sqlInsertCloset, closet.ID, closet.RoomID, closet.Summary)
	if err != nil {
		return err
	}

	for i, drawerID := range closet.DrawerIDs {
		_, err = tx.ExecContext(ctx, sqlInsertClosetDrawer, closet.ID, drawerID, i)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetClosets(ctx context.Context, roomID string) ([]*cobot.Closet, error) {
	rows, err := s.db.QueryContext(ctx, sqlSelectClosets, roomID)
	if err != nil {
		return nil, err
	}

	// Collect all closets first and close the rows cursor before running
	// sub-queries. With MaxOpenConns(1), nested queries on the same DB
	// would deadlock if the outer cursor is still open.
	var closets []*cobot.Closet
	for rows.Next() {
		var c cobot.Closet
		if err := rows.Scan(&c.ID, &c.RoomID, &c.Summary); err != nil {
			rows.Close()
			return nil, err
		}
		closets = append(closets, &c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, c := range closets {
		drawerRows, err := s.db.QueryContext(ctx, sqlSelectClosetDrawers, c.ID)
		if err != nil {
			return nil, err
		}
		for drawerRows.Next() {
			var did string
			if err := drawerRows.Scan(&did); err != nil {
				drawerRows.Close()
				return nil, err
			}
			c.DrawerIDs = append(c.DrawerIDs, did)
		}
		drawerRows.Close()
	}

	return closets, nil
}

// --- scan helpers ---

func scanWing(row *sql.Row) (*cobot.Wing, error) {
	var w cobot.Wing
	var kwJSON string
	if err := row.Scan(&w.ID, &w.Name, &w.Type, &kwJSON); err != nil {
		return nil, err
	}
	if kwJSON != "" && kwJSON != "[]" {
		if err := json.Unmarshal([]byte(kwJSON), &w.Keywords); err != nil {
			return nil, fmt.Errorf("unmarshal keywords: %w", err)
		}
	}
	return &w, nil
}

func scanWingRows(rows *sql.Rows) (*cobot.Wing, error) {
	var w cobot.Wing
	var kwJSON string
	if err := rows.Scan(&w.ID, &w.Name, &w.Type, &kwJSON); err != nil {
		return nil, err
	}
	if kwJSON != "" && kwJSON != "[]" {
		if err := json.Unmarshal([]byte(kwJSON), &w.Keywords); err != nil {
			return nil, fmt.Errorf("unmarshal keywords: %w", err)
		}
	}
	return &w, nil
}
