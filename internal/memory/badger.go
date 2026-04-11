package memory

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/dgraph-io/badger/v4"
)

const (
	prefixWing   = "wing:"
	prefixRoom   = "room:"
	prefixDrawer = "drawer:"
	prefixCloset = "closet:"
	prefixKG     = "kg:"
)

func openBadger(dir string) (*badger.DB, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil
	return badger.Open(opts)
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
