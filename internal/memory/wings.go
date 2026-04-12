package memory

import (
	"context"
	"sync"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

var (
	wingMu sync.Mutex
	roomMu sync.Mutex
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

func (s *Store) GetWingByName(ctx context.Context, name string) (*cobot.Wing, error) {
	wings, err := s.GetWings(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range wings {
		if w.Name == name {
			return w, nil
		}
	}
	return nil, nil
}

func (s *Store) CreateWingIfNotExists(ctx context.Context, name string) (string, error) {
	wingMu.Lock()
	defer wingMu.Unlock()

	if existing, err := s.GetWingByName(ctx, name); err != nil {
		return "", err
	} else if existing != nil {
		return existing.ID, nil
	}

	wing := &cobot.Wing{ID: newID(), Name: name}
	if err := s.CreateWing(ctx, wing); err != nil {
		return "", err
	}
	return wing.ID, nil
}
