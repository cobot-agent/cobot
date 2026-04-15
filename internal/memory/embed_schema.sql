CREATE TABLE IF NOT EXISTS wings (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL DEFAULT '',
	keywords_json TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS rooms (
	id TEXT PRIMARY KEY,
	wing_id TEXT NOT NULL REFERENCES wings(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	hall_type TEXT NOT NULL DEFAULT '',
	UNIQUE (wing_id, name)
);

CREATE TABLE IF NOT EXISTS drawers (
	id TEXT PRIMARY KEY,
	room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
	content TEXT NOT NULL,
	tag TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS closets (
	id TEXT PRIMARY KEY,
	room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
	summary TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS closet_drawers (
	closet_id TEXT NOT NULL REFERENCES closets(id) ON DELETE CASCADE,
	drawer_id TEXT NOT NULL REFERENCES drawers(id) ON DELETE CASCADE,
	position INTEGER NOT NULL,
	PRIMARY KEY (closet_id, drawer_id)
);

CREATE INDEX IF NOT EXISTS idx_rooms_wing_id ON rooms(wing_id);
CREATE INDEX IF NOT EXISTS idx_drawers_room_id ON drawers(room_id);
CREATE INDEX IF NOT EXISTS idx_drawers_created_at ON drawers(created_at);
CREATE INDEX IF NOT EXISTS idx_closet_drawers_drawer_id ON closet_drawers(drawer_id);

CREATE VIRTUAL TABLE IF NOT EXISTS drawer_fts USING fts5(
	content,
	content='drawers',
	content_rowid='rowid',
	tokenize='unicode61 remove_diacritics 2'
);

-- Triggers to keep FTS index in sync with drawers table.
CREATE TRIGGER IF NOT EXISTS drawer_fts_insert AFTER INSERT ON drawers BEGIN
	INSERT INTO drawer_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS drawer_fts_delete BEFORE DELETE ON drawers BEGIN
	INSERT INTO drawer_fts(drawer_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS drawer_fts_update AFTER UPDATE ON drawers BEGIN
	INSERT INTO drawer_fts(drawer_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
	INSERT INTO drawer_fts(rowid, content) VALUES (new.rowid, new.content);
END;
