package memory

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/blevesearch/bleve/v2"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

type Store struct {
	db       *badger.DB
	bleveDir string
	bleveIdx bleve.Index
}

func OpenStore(memoryDir string) (*Store, error) {
	dbPath := filepath.Join(memoryDir, "badger")
	db, err := openBadger(dbPath)
	if err != nil {
		return nil, err
	}
	bleveDir := filepath.Join(memoryDir, "bleve")
	os.MkdirAll(bleveDir, 0755)
	s := &Store{db: db, bleveDir: bleveDir}
	idx, err := s.openIndex()
	if err != nil {
		db.Close()
		return nil, err
	}
	s.bleveIdx = idx
	return s, nil
}

func (s *Store) Close() error {
	if s.bleveIdx != nil {
		s.bleveIdx.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) GetWings(ctx context.Context) ([]*cobot.Wing, error) {
	var wings []*cobot.Wing
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(prefixWing)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var w cobot.Wing
			if err := item.Value(func(val []byte) error {
				return unmarshal(val, &w)
			}); err != nil {
				return err
			}
			wings = append(wings, &w)
		}
		return nil
	})
	return wings, err
}

func (s *Store) Store(ctx context.Context, content string, wingID, roomID string) (string, error) {
	id, err := s.AddDrawer(ctx, wingID, roomID, content)
	if err != nil {
		return "", err
	}

	room, err := s.GetRoom(ctx, wingID, roomID)
	if err != nil {
		s.DeleteDrawer(ctx, id)
		return "", err
	}

	if err := s.indexDrawer(ctx, &drawerDoc{
		ID:        id,
		Content:   content,
		WingID:    wingID,
		RoomID:    roomID,
		HallType:  room.HallType,
		CreatedAt: time.Now(),
	}); err != nil {
		s.DeleteDrawer(ctx, id)
		return "", err
	}

	return id, nil
}

func (s *Store) Search(ctx context.Context, query *cobot.SearchQuery) ([]*cobot.SearchResult, error) {
	return s.searchDrawers(ctx, query)
}
