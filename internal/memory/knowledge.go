package memory

import (
	"context"
	"sort"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

// tripleKey builds a collision-free Badger key for a knowledge triple using
// NUL bytes as separators. Although Go strings can contain \x00, the values
// stored in the knowledge graph are user-visible identifiers that never
// include NUL, so the key space is unambiguous.
func tripleKey(subject, predicate, object string) string {
	return prefixKG + subject + "\x00" + predicate + "\x00" + object
}

func (s *Store) AddTriple(ctx context.Context, triple *cobot.Triple) error {
	if triple.ValidFrom.IsZero() {
		triple.ValidFrom = time.Now()
	}
	key := tripleKey(triple.Subject, triple.Predicate, triple.Object)
	data, err := marshal(triple)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
}

func (s *Store) Invalidate(ctx context.Context, subject, predicate, object string, ended time.Time) error {
	key := tripleKey(subject, predicate, object)
	return s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		var triple cobot.Triple
		if err := item.Value(func(val []byte) error {
			return unmarshal(val, &triple)
		}); err != nil {
			return err
		}
		triple.ValidTo = &ended
		data, err := marshal(&triple)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), data)
	})
}

func (s *Store) Query(ctx context.Context, entity string, asOf *time.Time) ([]*cobot.Triple, error) {
	prefix := []byte(prefixKG + entity + "\x00")
	var results []*cobot.Triple
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var triple cobot.Triple
			if err := item.Value(func(val []byte) error {
				return unmarshal(val, &triple)
			}); err != nil {
				return err
			}
			if asOf != nil {
				if triple.ValidFrom.After(*asOf) {
					continue
				}
				if triple.ValidTo != nil && triple.ValidTo.Before(*asOf) {
					continue
				}
			}
			results = append(results, &triple)
		}
		return nil
	})
	return results, err
}

func (s *Store) Timeline(ctx context.Context, entity string) ([]*cobot.Triple, error) {
	prefix := []byte(prefixKG + entity + "\x00")
	var results []*cobot.Triple
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var triple cobot.Triple
			if err := item.Value(func(val []byte) error {
				return unmarshal(val, &triple)
			}); err != nil {
				return err
			}
			results = append(results, &triple)
		}
		return nil
	})
	sort.Slice(results, func(i, j int) bool {
		return results[i].ValidFrom.Before(results[j].ValidFrom)
	})
	return results, err
}
