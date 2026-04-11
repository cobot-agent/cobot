package memory

import (
	"context"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

func (s *Store) CreateCloset(ctx context.Context, closet *cobot.Closet) error {
	if closet.ID == "" {
		closet.ID = newID()
	}
	data, err := marshal(closet)
	if err != nil {
		return err
	}
	key := prefixCloset + closet.RoomID + ":" + closet.ID
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
}

func (s *Store) GetClosets(ctx context.Context, roomID string) ([]*cobot.Closet, error) {
	var closets []*cobot.Closet
	prefix := prefixCloset + roomID + ":"
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			var c cobot.Closet
			if err := item.Value(func(val []byte) error {
				return unmarshal(val, &c)
			}); err != nil {
				return err
			}
			closets = append(closets, &c)
		}
		return nil
	})
	return closets, err
}
