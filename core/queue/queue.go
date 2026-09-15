// Package queue contiene el dominio y las reglas FIFO de la cola musical.
package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rockola/core/catalog"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusPlaying   Status = "playing"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrEntryNotFound     = errors.New("elemento de cola no encontrado")
	ErrNoCurrentSong     = errors.New("no hay una canción reproduciéndose")
	ErrNoNextSong        = errors.New("no hay canciones pendientes")
	ErrInvalidTransition = errors.New("transición de estado de cola no válida")
)

// Entry une el estado persistido de la cola con los datos de su canción.
type Entry struct {
	ID          int64        `json:"id"`
	SongID      int64        `json:"song_id"`
	Position    int64        `json:"position"`
	Status      Status       `json:"status"`
	RequestedAt time.Time    `json:"requested_at"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
	Song        catalog.Song `json:"song"`
}

// Repository abstrae SQLite e incluye la operación transaccional que consume
// un crédito y agrega la canción a la cola.
type Repository interface {
	AddWithCredit(ctx context.Context, songID int64) (*Entry, error)
	List(ctx context.Context) ([]Entry, error)
	Remove(ctx context.Context, queueID int64) error
	Clear(ctx context.Context) error
	Current(ctx context.Context) (*Entry, error)
	Next(ctx context.Context) (*Entry, error)
	MarkPlaying(ctx context.Context, queueID int64) error
	MarkCompleted(ctx context.Context, queueID int64) error
	MarkCancelled(ctx context.Context, queueID int64) error
}

type Service struct {
	repo    Repository
	catalog *catalog.Service
}

func NewService(repo Repository, catalogService *catalog.Service) *Service {
	return &Service{repo: repo, catalog: catalogService}
}

// AddToQueue valida la canción y su archivo antes de consumir el crédito.
// El descuento y el INSERT de cola se ejecutan atómicamente en el repositorio.
func (s *Service) AddToQueue(ctx context.Context, songID int64) (*Entry, error) {
	song, err := s.catalog.GetSong(ctx, songID)
	if err != nil {
		return nil, err
	}
	if !song.Enabled {
		return nil, catalog.ErrSongNotFound
	}
	if err := s.catalog.ValidateMediaPath(song); err != nil {
		return nil, err
	}
	entry, err := s.repo.AddWithCredit(ctx, songID)
	if err != nil {
		return nil, fmt.Errorf("agregando canción a la cola: %w", err)
	}
	return entry, nil
}

func (s *Service) GetQueue(ctx context.Context) ([]Entry, error) {
	return s.repo.List(ctx)
}

func (s *Service) RemoveFromQueue(ctx context.Context, queueID int64) error {
	return s.repo.Remove(ctx, queueID)
}

func (s *Service) ClearQueue(ctx context.Context) error {
	return s.repo.Clear(ctx)
}

func (s *Service) GetCurrentSong(ctx context.Context) (*Entry, error) {
	return s.repo.Current(ctx)
}

func (s *Service) GetNextSong(ctx context.Context) (*Entry, error) {
	return s.repo.Next(ctx)
}

func (s *Service) MarkPlaying(ctx context.Context, queueID int64) error {
	return s.repo.MarkPlaying(ctx, queueID)
}

func (s *Service) MarkCompleted(ctx context.Context, queueID int64) error {
	return s.repo.MarkCompleted(ctx, queueID)
}

// MarkCancelled detiene una entrada que ya estaba reproduciéndose. Se usa
// para mantener consistente la cola cuando el player se detiene o falla.
func (s *Service) MarkCancelled(ctx context.Context, queueID int64) error {
	return s.repo.MarkCancelled(ctx, queueID)
}
