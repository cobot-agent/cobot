package memory

import (
	"context"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

func roomKey(room *cobot.Room) string {
	return prefixRoom + room.WingID + ":" + room.ID
}

func (s *Store) CreateRoom(ctx context.Context, room *cobot.Room) error {
	if room.ID == "" {
		room.ID = newID()
	}
	data, err := marshal(room)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(roomKey(room)), data)
	})
}

func (s *Store) GetRooms(ctx context.Context, wingID string) ([]*cobot.Room, error) {
	var rooms []*cobot.Room
	prefix := prefixRoom + wingID + ":"
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			var r cobot.Room
			if err := item.Value(func(val []byte) error {
				return unmarshal(val, &r)
			}); err != nil {
				return err
			}
			rooms = append(rooms, &r)
		}
		return nil
	})
	return rooms, err
}

func (s *Store) GetRoom(ctx context.Context, wingID, roomID string) (*cobot.Room, error) {
	var r cobot.Room
	key := prefixRoom + wingID + ":" + roomID
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return unmarshal(val, &r)
		})
	})
	if err != nil {
		return nil, err
	}
	return &r, nil
}
