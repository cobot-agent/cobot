package daemon

import (
	"context"
	"net"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"

	"github.com/cobot-agent/cobot/internal/memory"
	cobot "github.com/cobot-agent/cobot/pkg"
)

type Server struct {
	store *memory.Store
}

func NewServer(store *memory.Store) *Server {
	return &Server{store: store}
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	assigner := handler.Map{
		"memory.ping":                  handler.New(s.handlePing),
		"memory.store":                 handler.New(s.handleStore),
		"memory.search":                handler.New(s.handleSearch),
		"memory.getWings":              handler.New(s.handleGetWings),
		"memory.getWingByName":         handler.New(s.handleGetWingByName),
		"memory.createWingIfNotExists": handler.New(s.handleCreateWingIfNotExists),
		"memory.getRooms":              handler.New(s.handleGetRooms),
		"memory.getRoomByName":         handler.New(s.handleGetRoomByName),
		"memory.createRoomIfNotExists": handler.New(s.handleCreateRoomIfNotExists),
		"memory.wakeUp":                handler.New(s.handleWakeUp),
		"memory.l3DeepSearch":          handler.New(s.handleL3DeepSearch),
		"memory.autoSummarizeRoom":     handler.New(s.handleAutoSummarizeRoom),
	}

	srv := jrpc2.NewServer(assigner, &jrpc2.ServerOptions{
		Concurrency: 1,
	})

	go func() {
		<-ctx.Done()
		listener.Close()
		srv.Stop()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return srv.Wait()
			default:
				continue
			}
		}
		ch := channel.Line(conn, conn)
		srv.Start(ch)
	}
}

func (s *Server) handlePing(ctx context.Context) (string, error) {
	return "pong", nil
}

func (s *Server) handleStore(ctx context.Context, args struct {
	Content string `json:"content"`
	WingID  string `json:"wing_id"`
	RoomID  string `json:"room_id"`
}) (string, error) {
	return s.store.Store(ctx, args.Content, args.WingID, args.RoomID)
}

func (s *Server) handleSearch(ctx context.Context, args cobot.SearchQuery) ([]*cobot.SearchResult, error) {
	return s.store.Search(ctx, &args)
}

func (s *Server) handleGetWings(ctx context.Context) ([]*cobot.Wing, error) {
	return s.store.GetWings(ctx)
}

func (s *Server) handleGetWingByName(ctx context.Context, args struct {
	Name string `json:"name"`
}) (*cobot.Wing, error) {
	return s.store.GetWingByName(ctx, args.Name)
}

func (s *Server) handleCreateWingIfNotExists(ctx context.Context, args struct {
	Name string `json:"name"`
}) (string, error) {
	return s.store.CreateWingIfNotExists(ctx, args.Name)
}

func (s *Server) handleGetRooms(ctx context.Context, args struct {
	WingID string `json:"wing_id"`
}) ([]*cobot.Room, error) {
	return s.store.GetRooms(ctx, args.WingID)
}

func (s *Server) handleGetRoomByName(ctx context.Context, args struct {
	WingID string `json:"wing_id"`
	Name   string `json:"name"`
}) (*cobot.Room, error) {
	return s.store.GetRoomByName(ctx, args.WingID, args.Name)
}

func (s *Server) handleCreateRoomIfNotExists(ctx context.Context, args struct {
	WingID   string `json:"wing_id"`
	Name     string `json:"name"`
	HallType string `json:"hall_type"`
}) (string, error) {
	return s.store.CreateRoomIfNotExists(ctx, args.WingID, args.Name, args.HallType)
}

func (s *Server) handleWakeUp(ctx context.Context) (string, error) {
	return s.store.WakeUp(ctx)
}

func (s *Server) handleL3DeepSearch(ctx context.Context, args struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}) ([]*cobot.SearchResult, error) {
	return s.store.L3DeepSearch(ctx, args.Query, args.Limit)
}

func (s *Server) handleAutoSummarizeRoom(ctx context.Context, args struct {
	WingID string `json:"wing_id"`
	RoomID string `json:"room_id"`
}) (string, error) {
	if err := s.store.AutoSummarizeRoom(ctx, args.WingID, args.RoomID); err != nil {
		return "", err
	}
	return "ok", nil
}
