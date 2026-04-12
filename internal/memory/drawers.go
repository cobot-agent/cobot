package memory

import (
	"context"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

func (s *Store) AddDrawer(ctx context.Context, wingID, roomID, content string) (string, error) {
	id := newID()
	d := &cobot.Drawer{
		ID:        id,
		RoomID:    roomID,
		Content:   content,
		CreatedAt: time.Now(),
	}
	data, err := marshal(d)
	if err != nil {
		return "", err
	}
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(prefixDrawer+id), data)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) GetDrawer(ctx context.Context, id string) (*cobot.Drawer, error) {
	var d cobot.Drawer
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(prefixDrawer + id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return unmarshal(val, &d)
		})
	})
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) DeleteDrawer(ctx context.Context, id string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(prefixDrawer + id))
	})
}
