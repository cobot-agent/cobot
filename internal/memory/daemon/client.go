package daemon

import (
	"context"
	"net"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"

	"github.com/cobot-agent/cobot/internal/memory"
	cobot "github.com/cobot-agent/cobot/pkg"
)

type RemoteStore struct {
	client *jrpc2.Client
	conn   net.Conn
}

func Dial(ctx context.Context, socketPath string) (*RemoteStore, error) {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, err
	}
	ch := channel.Line(conn, conn)
	cli := jrpc2.NewClient(ch, nil)
	return &RemoteStore{client: cli, conn: conn}, nil
}

func (r *RemoteStore) Store(ctx context.Context, content string, wingID, roomID string) (string, error) {
	var result string
	err := r.client.CallResult(ctx, "memory.store", map[string]string{
		"content": content,
		"wing_id": wingID,
		"room_id": roomID,
	}, &result)
	return result, err
}

func (r *RemoteStore) Search(ctx context.Context, query *cobot.SearchQuery) ([]*cobot.SearchResult, error) {
	var results []*cobot.SearchResult
	err := r.client.CallResult(ctx, "memory.search", query, &results)
	return results, err
}

func (r *RemoteStore) GetWings(ctx context.Context) ([]*cobot.Wing, error) {
	var wings []*cobot.Wing
	err := r.client.CallResult(ctx, "memory.getWings", nil, &wings)
	return wings, err
}

func (r *RemoteStore) GetWingByName(ctx context.Context, name string) (*cobot.Wing, error) {
	var w cobot.Wing
	err := r.client.CallResult(ctx, "memory.getWingByName", map[string]string{"name": name}, &w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *RemoteStore) CreateWingIfNotExists(ctx context.Context, name string) (string, error) {
	var id string
	err := r.client.CallResult(ctx, "memory.createWingIfNotExists", map[string]string{"name": name}, &id)
	return id, err
}

func (r *RemoteStore) GetRooms(ctx context.Context, wingID string) ([]*cobot.Room, error) {
	var rooms []*cobot.Room
	err := r.client.CallResult(ctx, "memory.getRooms", map[string]string{"wing_id": wingID}, &rooms)
	return rooms, err
}

func (r *RemoteStore) GetRoomByName(ctx context.Context, wingID, name string) (*cobot.Room, error) {
	var room cobot.Room
	err := r.client.CallResult(ctx, "memory.getRoomByName", map[string]string{"wing_id": wingID, "name": name}, &room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RemoteStore) CreateRoomIfNotExists(ctx context.Context, wingID, name, hallType string) (string, error) {
	var id string
	err := r.client.CallResult(ctx, "memory.createRoomIfNotExists", map[string]string{
		"wing_id":   wingID,
		"name":      name,
		"hall_type": hallType,
	}, &id)
	return id, err
}

func (r *RemoteStore) WakeUp(ctx context.Context) (string, error) {
	var result string
	err := r.client.CallResult(ctx, "memory.wakeUp", nil, &result)
	return result, err
}

func (r *RemoteStore) L3DeepSearch(ctx context.Context, query string, limit int) ([]*cobot.SearchResult, error) {
	var results []*cobot.SearchResult
	err := r.client.CallResult(ctx, "memory.l3DeepSearch", map[string]any{
		"query": query,
		"limit": limit,
	}, &results)
	return results, err
}

func (r *RemoteStore) AutoSummarizeRoom(ctx context.Context, wingID, roomID string) error {
	var result string
	return r.client.CallResult(ctx, "memory.autoSummarizeRoom", map[string]string{
		"wing_id": wingID,
		"room_id": roomID,
	}, &result)
}

func (r *RemoteStore) Close() error {
	if r.client != nil {
		r.client.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

var _ memory.Client = (*RemoteStore)(nil)
