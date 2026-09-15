// Package catalog contiene el dominio del catálogo musical:
// el modelo Song, la interfaz de persistencia y el servicio de negocio.
package catalog

import (
	"context"
	"time"
)

// Song representa una canción del catálogo. El archivo de audio vive en el
// filesystem; la base de datos solo almacena su ruta (FilePath).
type Song struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	Album     string    `json:"album"`
	Genre     string    `json:"genre"`
	FilePath  string    `json:"file_path"`
	CoverPath string    `json:"cover_path"`
	Duration  int       `json:"duration"` // segundos
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Repository define las operaciones de persistencia del catálogo.
// La implementación concreta (SQLite con LIKE, y en el futuro FTS5)
// vive en core/storage; el dominio no conoce SQL.
type Repository interface {
	Insert(ctx context.Context, s *Song) error
	GetByID(ctx context.Context, id int64) (*Song, error)
	ExistsByPath(ctx context.Context, path string) (bool, error)
	// Search busca por título, artista, álbum o género.
	Search(ctx context.Context, query string) ([]Song, error)
	ListAll(ctx context.Context) ([]Song, error)
	ListArtists(ctx context.Context) ([]string, error)
	ListAlbums(ctx context.Context) ([]string, error)
	ListGenres(ctx context.Context) ([]string, error)
}
