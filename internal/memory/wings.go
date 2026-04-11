package memory

import (
	"context"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

func (s *Store) CreateWing(ctx context.Context, wing *cobot.Wing) error {
	if wing.ID == "" {
		wing.ID = newID()
	}
	data, err := marshal(wing)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(prefixWing+wing.ID), data)
	})
}

func (s *Store) GetWing(ctx context.Context, id string) (*cobot.Wing, error) {
	var w cobot.Wing
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(prefixWing + id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return unmarshal(val, &w)
		})
	})
	if err != nil {
		return nil, err
	}
	return &w, nil
}
