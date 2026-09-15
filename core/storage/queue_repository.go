package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rockola/core/credits"
	"rockola/core/queue"
)

type QueueRepository struct {
	db *sql.DB
}

var _ queue.Repository = (*QueueRepository)(nil)

func NewQueueRepository(db *sql.DB) *QueueRepository {
	return &QueueRepository{db: db}
}

const queueSongColumns = `q.id, q.song_id, q.position, q.status,
	q.requested_at, q.started_at, q.finished_at,
	s.id, s.title, s.artist, s.album, s.genre, s.file_path, s.cover_path,
	s.duration, s.enabled, s.created_at, s.updated_at`

func (r *QueueRepository) AddWithCredit(ctx context.Context, songID int64) (*queue.Entry, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("iniciando transacción: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE credits SET balance = balance - 1, updated_at = ?
		WHERE id = 1 AND balance > 0`, formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("consumiendo saldo: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("verificando saldo: %w", err)
	}
	if affected == 0 {
		return nil, credits.ErrInsufficientCredit
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_transactions (amount, credits, type, created_at)
		VALUES (0, -1, 'consume', ?)`, formatTime(now)); err != nil {
		return nil, fmt.Errorf("registrando consumo: %w", err)
	}

	result, err = tx.ExecContext(ctx, `
		INSERT INTO queue (song_id, position, status, requested_at)
		VALUES (?, COALESCE((SELECT MAX(position) + 1 FROM queue), 1), 'pending', ?)`,
		songID, formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("insertando en cola: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("obteniendo id de cola: %w", err)
	}
	entry, err := getQueueEntry(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("confirmando cola: %w", err)
	}
	return entry, nil
}

// List devuelve la cola activa; los completados/cancelados siguen persistidos
// como historial, pero no forman parte de la cola visible.
func (r *QueueRepository) List(ctx context.Context) ([]queue.Entry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+queueSongColumns+`
		FROM queue q JOIN songs s ON s.id = q.song_id
		WHERE q.status IN ('pending', 'playing')
		ORDER BY q.position`)
	if err != nil {
		return nil, fmt.Errorf("consultando cola: %w", err)
	}
	defer rows.Close()

	var entries []queue.Entry
	for rows.Next() {
		entry, err := scanQueueEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("leyendo cola: %w", err)
		}
		entries = append(entries, *entry)
	}
	return entries, rows.Err()
}

func (r *QueueRepository) Remove(ctx context.Context, queueID int64) error {
	return updateQueueStatus(ctx, r.db, queueID, queue.StatusPending, queue.StatusCancelled)
}

func (r *QueueRepository) Clear(ctx context.Context) error {
	now := formatTime(time.Now().UTC())
	_, err := r.db.ExecContext(ctx, `
		UPDATE queue SET status = 'cancelled', finished_at = ?
		WHERE status IN ('pending', 'playing')`, now)
	if err != nil {
		return fmt.Errorf("limpiando cola: %w", err)
	}
	return nil
}

func (r *QueueRepository) Current(ctx context.Context) (*queue.Entry, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+queueSongColumns+`
		FROM queue q JOIN songs s ON s.id = q.song_id
		WHERE q.status = 'playing' ORDER BY q.position LIMIT 1`)
	entry, err := scanQueueEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, queue.ErrNoCurrentSong
	}
	if err != nil {
		return nil, fmt.Errorf("consultando canción actual: %w", err)
	}
	return entry, nil
}

func (r *QueueRepository) Next(ctx context.Context) (*queue.Entry, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+queueSongColumns+`
		FROM queue q JOIN songs s ON s.id = q.song_id
		WHERE q.status = 'pending' ORDER BY q.position LIMIT 1`)
	entry, err := scanQueueEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, queue.ErrNoNextSong
	}
	if err != nil {
		return nil, fmt.Errorf("consultando siguiente canción: %w", err)
	}
	return entry, nil
}

func (r *QueueRepository) MarkPlaying(ctx context.Context, queueID int64) error {
	// NOT EXISTS protege la regla de una sola canción reproduciéndose.
	now := formatTime(time.Now().UTC())
	result, err := r.db.ExecContext(ctx, `
		UPDATE queue SET status = 'playing', started_at = ?
		WHERE id = ? AND status = 'pending'
		  AND NOT EXISTS (SELECT 1 FROM queue WHERE status = 'playing')`, now, queueID)
	if err != nil {
		return fmt.Errorf("marcando en reproducción: %w", err)
	}
	return requireAffected(result)
}

func (r *QueueRepository) MarkCompleted(ctx context.Context, queueID int64) error {
	return updateQueueStatus(ctx, r.db, queueID, queue.StatusPlaying, queue.StatusCompleted)
}

func (r *QueueRepository) MarkCancelled(ctx context.Context, queueID int64) error {
	return updateQueueStatus(ctx, r.db, queueID, queue.StatusPlaying, queue.StatusCancelled)
}

func updateQueueStatus(ctx context.Context, db *sql.DB, id int64, from, to queue.Status) error {
	result, err := db.ExecContext(ctx, `
		UPDATE queue SET status = ?, finished_at = ?
		WHERE id = ? AND status = ?`, to, formatTime(time.Now().UTC()), id, from)
	if err != nil {
		return fmt.Errorf("actualizando estado de cola: %w", err)
	}
	return requireAffected(result)
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verificando actualización: %w", err)
	}
	if affected == 0 {
		return queue.ErrInvalidTransition
	}
	return nil
}

func getQueueEntry(ctx context.Context, q queryRower, id int64) (*queue.Entry, error) {
	entry, err := scanQueueEntry(q.QueryRowContext(ctx, `SELECT `+queueSongColumns+`
		FROM queue q JOIN songs s ON s.id = q.song_id WHERE q.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, queue.ErrEntryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consultando elemento de cola: %w", err)
	}
	return entry, nil
}

func scanQueueEntry(row rowScanner) (*queue.Entry, error) {
	var (
		entry                        queue.Entry
		status                       string
		requestedAt                  string
		startedAt, finishedAt        sql.NullString
		enabled                      int
		songCreatedAt, songUpdatedAt string
	)
	err := row.Scan(
		&entry.ID, &entry.SongID, &entry.Position, &status,
		&requestedAt, &startedAt, &finishedAt,
		&entry.Song.ID, &entry.Song.Title, &entry.Song.Artist,
		&entry.Song.Album, &entry.Song.Genre, &entry.Song.FilePath,
		&entry.Song.CoverPath, &entry.Song.Duration, &enabled,
		&songCreatedAt, &songUpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	entry.Status = queue.Status(status)
	entry.RequestedAt, _ = time.Parse(time.RFC3339Nano, requestedAt)
	entry.StartedAt = parseNullableTime(startedAt)
	entry.FinishedAt = parseNullableTime(finishedAt)
	entry.Song.Enabled = enabled != 0
	entry.Song.CreatedAt, _ = time.Parse(time.RFC3339Nano, songCreatedAt)
	entry.Song.UpdatedAt, _ = time.Parse(time.RFC3339Nano, songUpdatedAt)
	return &entry, nil
}

func parseNullableTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &t
}
