package catalog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Errores de dominio del catálogo.
var (
	ErrSongNotFound  = errors.New("canción no encontrada")
	ErrMediaNotFound = errors.New("el archivo de audio no existe en el filesystem")
	ErrEmptyQuery    = errors.New("la búsqueda no puede estar vacía")
)

// Service expone las operaciones de negocio del catálogo.
type Service struct {
	repo Repository
	log  *slog.Logger
}

// NewService crea el servicio de catálogo.
func NewService(repo Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// SearchSongs busca canciones por título, artista, álbum o género.
func (s *Service) SearchSongs(ctx context.Context, query string) ([]Song, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}
	songs, err := s.repo.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("buscando canciones: %w", err)
	}
	s.log.Debug("búsqueda ejecutada", "query", query, "resultados", len(songs))
	return songs, nil
}

// GetSong devuelve una canción por su ID.
func (s *Service) GetSong(ctx context.Context, id int64) (*Song, error) {
	song, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return song, nil
}

// ListSongs devuelve todo el catálogo habilitado.
func (s *Service) ListSongs(ctx context.Context) ([]Song, error) {
	return s.repo.ListAll(ctx)
}

// ListArtists devuelve los artistas únicos del catálogo.
func (s *Service) ListArtists(ctx context.Context) ([]string, error) {
	return s.repo.ListArtists(ctx)
}

// ListAlbums devuelve los álbumes únicos del catálogo.
func (s *Service) ListAlbums(ctx context.Context) ([]string, error) {
	return s.repo.ListAlbums(ctx)
}

// ListGenres devuelve los géneros únicos del catálogo.
func (s *Service) ListGenres(ctx context.Context) ([]string, error) {
	return s.repo.ListGenres(ctx)
}

// ValidateMediaPath verifica que el archivo de audio de la canción exista
// en el filesystem. Devuelve ErrMediaNotFound si no existe.
func (s *Service) ValidateMediaPath(song *Song) error {
	info, err := os.Stat(song.FilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrMediaNotFound, song.FilePath)
		}
		return fmt.Errorf("verificando archivo %s: %w", song.FilePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%w: %s es un directorio", ErrMediaNotFound, song.FilePath)
	}
	return nil
}
