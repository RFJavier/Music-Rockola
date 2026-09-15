package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rockola/core/catalog"
)

// SongRepository implementa catalog.Repository sobre SQLite.
//
// La búsqueda usa LIKE (suficiente para el MVP). Para migrar a FTS5 solo
// hay que cambiar el método Search de esta implementación; el dominio y la
// API no se ven afectados porque dependen de la interfaz catalog.Repository.
type SongRepository struct {
	db *sql.DB
}

// Verificación en compilación de que se cumple la interfaz del dominio.
var _ catalog.Repository = (*SongRepository)(nil)

// NewSongRepository crea el repositorio de canciones.
func NewSongRepository(db *sql.DB) *SongRepository {
	return &SongRepository{db: db}
}

const songColumns = `id, title, artist, album, genre, file_path, cover_path,
	duration, enabled, created_at, updated_at`

// Insert registra una canción nueva y asigna su ID.
func (r *SongRepository) Insert(ctx context.Context, s *catalog.Song) error {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO songs (title, artist, album, genre, file_path, cover_path,
			duration, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Title, s.Artist, s.Album, s.Genre, s.FilePath, s.CoverPath,
		s.Duration, boolToInt(s.Enabled),
		s.CreatedAt.UTC().Format(time.RFC3339),
		s.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insertando canción: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("obteniendo id insertado: %w", err)
	}
	s.ID = id
	return nil
}

// GetByID devuelve una canción por ID, o catalog.ErrSongNotFound.
func (r *SongRepository) GetByID(ctx context.Context, id int64) (*catalog.Song, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+songColumns+` FROM songs WHERE id = ?`, id)
	song, err := scanSong(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, catalog.ErrSongNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consultando canción %d: %w", id, err)
	}
	return song, nil
}

// ExistsByPath indica si ya existe una canción con esa ruta de archivo.
func (r *SongRepository) ExistsByPath(ctx context.Context, path string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM songs WHERE file_path = ?`, path).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("verificando existencia de %s: %w", path, err)
	}
	return true, nil
}

// Search busca canciones habilitadas por título, artista, álbum o género.
func (r *SongRepository) Search(ctx context.Context, query string) ([]catalog.Song, error) {
	pattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+songColumns+`
		FROM songs
		WHERE enabled = 1
		  AND (title LIKE ? OR artist LIKE ? OR album LIKE ? OR genre LIKE ?)
		ORDER BY artist, album, title`,
		pattern, pattern, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("buscando %q: %w", query, err)
	}
	defer rows.Close()
	return scanSongs(rows)
}

// ListAll devuelve todas las canciones habilitadas.
func (r *SongRepository) ListAll(ctx context.Context) ([]catalog.Song, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+songColumns+`
		FROM songs
		WHERE enabled = 1
		ORDER BY artist, album, title`)
	if err != nil {
		return nil, fmt.Errorf("listando canciones: %w", err)
	}
	defer rows.Close()
	return scanSongs(rows)
}

// ListArtists devuelve los artistas únicos (no vacíos).
func (r *SongRepository) ListArtists(ctx context.Context) ([]string, error) {
	return r.listDistinct(ctx, "artist")
}

// ListAlbums devuelve los álbumes únicos (no vacíos).
func (r *SongRepository) ListAlbums(ctx context.Context) ([]string, error) {
	return r.listDistinct(ctx, "album")
}

// ListGenres devuelve los géneros únicos (no vacíos).
func (r *SongRepository) ListGenres(ctx context.Context) ([]string, error) {
	return r.listDistinct(ctx, "genre")
}

// listDistinct devuelve los valores únicos de una columna del catálogo.
// `column` proviene solo de código interno (nunca de entrada del usuario).
func (r *SongRepository) listDistinct(ctx context.Context, column string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT `+column+`
		FROM songs
		WHERE enabled = 1 AND `+column+` != ''
		ORDER BY `+column)
	if err != nil {
		return nil, fmt.Errorf("listando %s: %w", column, err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("leyendo %s: %w", column, err)
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

// --- helpers de scan ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSong(row rowScanner) (*catalog.Song, error) {
	var (
		s                    catalog.Song
		enabled              int
		createdAt, updatedAt string
	)
	err := row.Scan(&s.ID, &s.Title, &s.Artist, &s.Album, &s.Genre,
		&s.FilePath, &s.CoverPath, &s.Duration, &enabled,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	s.Enabled = enabled != 0
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

func scanSongs(rows *sql.Rows) ([]catalog.Song, error) {
	var songs []catalog.Song
	for rows.Next() {
		s, err := scanSong(rows)
		if err != nil {
			return nil, fmt.Errorf("leyendo fila de canción: %w", err)
		}
		songs = append(songs, *s)
	}
	return songs, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
