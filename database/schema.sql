-- Esquema completo de la ROCKOLA.
-- Esquema utilizado por catálogo, cola FIFO y créditos simulados.

CREATE TABLE IF NOT EXISTS songs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    artist      TEXT    NOT NULL DEFAULT '',
    album       TEXT    NOT NULL DEFAULT '',
    genre       TEXT    NOT NULL DEFAULT '',
    file_path   TEXT    NOT NULL UNIQUE,
    cover_path  TEXT    NOT NULL DEFAULT '',
    duration    INTEGER NOT NULL DEFAULT 0, -- segundos
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_songs_title  ON songs (title);
CREATE INDEX IF NOT EXISTS idx_songs_artist ON songs (artist);
CREATE INDEX IF NOT EXISTS idx_songs_album  ON songs (album);
CREATE INDEX IF NOT EXISTS idx_songs_genre  ON songs (genre);

CREATE TABLE IF NOT EXISTS queue (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    song_id      INTEGER NOT NULL REFERENCES songs (id),
    position     INTEGER NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'playing', 'completed', 'cancelled')),
    requested_at TEXT    NOT NULL,
    started_at   TEXT,
    finished_at  TEXT
);

CREATE INDEX IF NOT EXISTS idx_queue_status ON queue (status, position);
CREATE UNIQUE INDEX IF NOT EXISTS idx_queue_single_playing
ON queue (status) WHERE status = 'playing';

CREATE TABLE IF NOT EXISTS credits (
    id         INTEGER PRIMARY KEY CHECK (id = 1), -- fila única
    balance    INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS credit_transactions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    amount     INTEGER NOT NULL, -- dinero simulado
    credits    INTEGER NOT NULL, -- créditos otorgados/consumidos (+/-)
    type       TEXT    NOT NULL CHECK (type IN ('add', 'consume')),
    created_at TEXT    NOT NULL
);

-- Fila inicial de créditos.
INSERT OR IGNORE INTO credits (id, balance, updated_at)
VALUES (1, 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
