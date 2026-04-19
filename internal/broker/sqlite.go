package brokersqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/cobot-agent/cobot/pkg/broker"
	_ "modernc.org/sqlite"
)

// sqliteTimeFmt matches SQLite strftime('%%Y-%%m-%%d %%H:%%M:%%f', 'now') output so text comparisons work correctly.
const sqliteTimeFmt = "2006-01-02 15:04:05.000000"

// sessionTTL is the duration after which a session without heartbeat is
// considered dead. Used by ListByChannel, ListAll and Cleanup.
const sessionTTL = 60 * time.Second

// SQLiteBroker implements the broker.Broker interface using SQLite WAL mode
// for multi-process coordination.
// It corresponds to a single shared coord.db file (usually placed at <workspace>/cron/coord.db).
type SQLiteBroker struct {
	db *sql.DB
}

// NewSQLiteBroker opens or creates the coordination database at dbPath.
func NewSQLiteBroker(dbPath string) (*SQLiteBroker, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create broker dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open broker db: %w", err)
	}
	db.SetMaxOpenConns(1)
	// WAL mode allows one writer and multiple readers concurrently.
	// _busy_timeout makes writers wait automatically instead of erroring.
	b := &SQLiteBroker{db: db}
	if err := b.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Purge receipts from dead sessions on startup so they don't accumulate
	// across restarts (session IDs change every restart).
	if err := b.cleanStaleReceipts(context.Background()); err != nil {
		slog.Warn("broker startup: failed to clean stale receipts", "error", err)
	}
	return b, nil
}

func (b *SQLiteBroker) initSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS locks (
	name TEXT PRIMARY KEY,
	holder TEXT NOT NULL,
	acquired_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	session_id TEXT PRIMARY KEY,
	channel_id TEXT NOT NULL,
	pid INTEGER NOT NULL,
	started_at TEXT NOT NULL,
	last_heartbeat TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_channel ON sessions(channel_id);
CREATE INDEX IF NOT EXISTS idx_sessions_heartbeat ON sessions(last_heartbeat);

CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	topic TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	payload BLOB NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel_id, created_at);

CREATE TABLE IF NOT EXISTS receipts (
	message_id INTEGER NOT NULL,
	session_id TEXT NOT NULL,
	acked_at TEXT NOT NULL,
	PRIMARY KEY (message_id, session_id)
);
`
	_, err := b.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("init broker schema: %w", err)
	}
	return nil
}

// --- Lock implementation ---

func (b *SQLiteBroker) TryAcquire(ctx context.Context, name, holder string, ttl time.Duration) (bool, error) {
	now := time.Now().UTC()
	expires := now.Add(ttl)

	var actual string
	err := b.db.QueryRowContext(ctx, `
		INSERT INTO locks (name, holder, acquired_at, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			holder = excluded.holder,
			acquired_at = excluded.acquired_at,
			expires_at = excluded.expires_at
		WHERE locks.expires_at < ?
		RETURNING holder;
	`, name, holder, now.Format(sqliteTimeFmt), expires.Format(sqliteTimeFmt), now.Format(sqliteTimeFmt)).Scan(&actual)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return actual == holder, nil
}

func (b *SQLiteBroker) Renew(ctx context.Context, name, holder string, ttl time.Duration) error {
	now := time.Now().UTC()
	expires := now.Add(ttl)
	res, err := b.db.ExecContext(ctx, `
		UPDATE locks SET expires_at = ?
		WHERE name = ? AND holder = ? AND expires_at > ?;
	`, expires.Format(sqliteTimeFmt), name, holder, now.Format(sqliteTimeFmt))
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("lock %q not held by %q or expired", name, holder)
	}
	return nil
}

func (b *SQLiteBroker) Release(ctx context.Context, name, holder string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM locks WHERE name = ? AND holder = ?`, name, holder)
	return err
}

// --- SessionRegistry implementation ---

func (b *SQLiteBroker) Register(ctx context.Context, info *broker.SessionInfo) error {
	_, err := b.db.ExecContext(ctx, `
		INSERT INTO sessions (session_id, channel_id, pid, started_at, last_heartbeat)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			channel_id = excluded.channel_id,
			pid = excluded.pid,
			last_heartbeat = excluded.last_heartbeat;
	`, info.ID, info.ChannelID, info.PID, info.StartedAt.Format(sqliteTimeFmt), time.Now().UTC().Format(sqliteTimeFmt))
	return err
}

func (b *SQLiteBroker) Unregister(ctx context.Context, sessionID string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM sessions WHERE session_id = ?`, sessionID)
	return err
}

func (b *SQLiteBroker) Heartbeat(ctx context.Context, sessionID string) error {
	_, err := b.db.ExecContext(ctx, `
		UPDATE sessions SET last_heartbeat = ? WHERE session_id = ?;
	`, time.Now().UTC().Format(sqliteTimeFmt), sessionID)
	return err
}

func (b *SQLiteBroker) ListByChannel(ctx context.Context, channelID string) ([]*broker.SessionInfo, error) {
	threshold := time.Now().UTC().Add(-sessionTTL).Format(sqliteTimeFmt)
	return b.listSessions(ctx, `SELECT session_id, channel_id, pid, started_at FROM sessions
		WHERE channel_id = ? AND last_heartbeat > ?`, channelID, threshold)
}

func (b *SQLiteBroker) ListAll(ctx context.Context) ([]*broker.SessionInfo, error) {
	threshold := time.Now().UTC().Add(-sessionTTL).Format(sqliteTimeFmt)
	return b.listSessions(ctx, `SELECT session_id, channel_id, pid, started_at FROM sessions
		WHERE last_heartbeat > ?`, threshold)
}

func (b *SQLiteBroker) listSessions(ctx context.Context, query string, args ...any) ([]*broker.SessionInfo, error) {
	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*broker.SessionInfo
	for rows.Next() {
		var s broker.SessionInfo
		var startedAt string
		if err := rows.Scan(&s.ID, &s.ChannelID, &s.PID, &startedAt); err != nil {
			return nil, err
		}
		s.StartedAt, err = time.Parse(sqliteTimeFmt, startedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, &s)
	}
	return results, rows.Err()
}

// --- PubSub implementation ---

func (b *SQLiteBroker) Publish(ctx context.Context, msg *broker.Message) error {
	_, err := b.db.ExecContext(ctx, `
		INSERT INTO messages (topic, channel_id, payload, created_at)
		VALUES (?, ?, ?, ?);
	`, msg.Topic, msg.ChannelID, msg.Payload, msg.CreatedAt.Format(sqliteTimeFmt))
	return err
}

func (b *SQLiteBroker) Consume(ctx context.Context, topic, channelID, sessionID string, limit int) ([]*broker.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if channelID == "" {
		rows, err = b.db.QueryContext(ctx, `
			SELECT m.id, m.topic, m.channel_id, m.payload, m.created_at
			FROM messages m
			WHERE m.topic = ?
			  AND NOT EXISTS (
				  SELECT 1 FROM receipts r
				  WHERE r.message_id = m.id AND r.session_id = ?
			  )
			ORDER BY m.id ASC
			LIMIT ?;
		`, topic, sessionID, limit)
	} else {
		rows, err = b.db.QueryContext(ctx, `
			SELECT m.id, m.topic, m.channel_id, m.payload, m.created_at
			FROM messages m
			WHERE m.topic = ? AND m.channel_id = ?
			  AND NOT EXISTS (
				  SELECT 1 FROM receipts r
				  WHERE r.message_id = m.id AND r.session_id = ?
			  )
			ORDER BY m.id ASC
			LIMIT ?;
		`, topic, channelID, sessionID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*broker.Message
	for rows.Next() {
		var msg broker.Message
		var id int64
		var createdAt string
		if err := rows.Scan(&id, &msg.Topic, &msg.ChannelID, &msg.Payload, &createdAt); err != nil {
			return nil, err
		}
		msg.ID = fmt.Sprintf("%d", id)
		msg.CreatedAt, err = time.Parse(sqliteTimeFmt, createdAt)
		if err != nil {
			return nil, err
		}
		results = append(results, &msg)
	}
	return results, rows.Err()
}

func (b *SQLiteBroker) Ack(ctx context.Context, msgID, sessionID string) error {
	_, err := b.db.ExecContext(ctx, `
		INSERT INTO receipts (message_id, session_id, acked_at)
		VALUES (?, ?, ?)
		ON CONFLICT DO NOTHING;
	`, msgID, sessionID, time.Now().UTC().Format(sqliteTimeFmt))
	return err
}

// --- Cleanup and Close ---

// cleanStaleReceipts removes receipts belonging to sessions that no longer
// exist. Called on startup to prevent bloat from previous session IDs.
func (b *SQLiteBroker) cleanStaleReceipts(ctx context.Context) error {
	_, err := b.db.ExecContext(ctx, `
		DELETE FROM receipts
		WHERE session_id NOT IN (SELECT session_id FROM sessions);
	`)
	return err
}

// Cleanup deletes expired sessions and fully delivered messages.
// Typically called periodically by the leader process.
func (b *SQLiteBroker) Cleanup(ctx context.Context) error {
	threshold := time.Now().UTC().Add(-sessionTTL).Format(sqliteTimeFmt)
	var errs []error

	// Delete sessions without heartbeat for the TTL duration.
	_, err := b.db.ExecContext(ctx, `DELETE FROM sessions WHERE last_heartbeat < ?;`, threshold)
	if err != nil {
		errs = append(errs, fmt.Errorf("cleanup sessions: %w", err))
	}

	// Delete messages that have been acked by all active sessions for that channel.
	// Batched to LIMIT 1000 to avoid holding write locks too long.
	_, err = b.db.ExecContext(ctx, `
		DELETE FROM messages
		WHERE id IN (
			SELECT m.id FROM messages m
			WHERE EXISTS (
				SELECT 1 FROM sessions s WHERE s.channel_id = m.channel_id
			)
			AND NOT EXISTS (
				SELECT 1 FROM sessions s
				WHERE s.channel_id = m.channel_id
				  AND NOT EXISTS (
					  SELECT 1 FROM receipts r
					  WHERE r.message_id = m.id AND r.session_id = s.session_id
				  )
			)
			LIMIT 1000
		);
	`)
	if err != nil {
		errs = append(errs, fmt.Errorf("cleanup messages: %w", err))
	}

	// Delete orphaned receipts (messages already deleted).
	_, err = b.db.ExecContext(ctx, `
		DELETE FROM receipts
		WHERE message_id NOT IN (SELECT id FROM messages);
	`)
	if err != nil {
		errs = append(errs, fmt.Errorf("cleanup orphaned receipts: %w", err))
	}

	return errors.Join(errs...)
}

// Close closes the SQLite connection.
func (b *SQLiteBroker) Close() error {
	return b.db.Close()
}

// Ensure SQLiteBroker satisfies the Broker interface.
var _ broker.Broker = (*SQLiteBroker)(nil)
