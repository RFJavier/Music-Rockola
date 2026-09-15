package playback

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"rockola/core/catalog"
	queuecore "rockola/core/queue"
)

var (
	ErrAlreadyPlaying = errors.New("el reproductor ya está reproduciendo")
	ErrNotPlaying     = errors.New("el reproductor no está reproduciendo")
	ErrNotPaused      = errors.New("el reproductor no está pausado")
)

type Status string

const (
	StatusStopped Status = "stopped"
	StatusPlaying Status = "playing"
	StatusPaused  Status = "paused"
	StatusEnded   Status = "ended"
)

// State es una representación JSON estable; las duraciones se expresan en
// milisegundos para evitar serializar time.Duration como nanosegundos.
type State struct {
	Status     Status           `json:"status"`
	QueueEntry *queuecore.Entry `json:"queue_entry,omitempty"`
	PositionMS int64            `json:"position_ms"`
	DurationMS int64            `json:"duration_ms"`
	Volume     float64          `json:"volume"`
}

// Service coordina el puerto multimedia con las transiciones de Queue. Esta
// fase permite avance manual; el monitor de fin automático se agrega en FASE 6.
type Service struct {
	mu      sync.Mutex
	player  MediaPlayer
	queue   *queuecore.Service
	catalog *catalog.Service
	status  Status
	current *queuecore.Entry
	volume  float64
}

func NewService(player MediaPlayer, queueService *queuecore.Service, catalogService *catalog.Service) *Service {
	return &Service{
		player: player, queue: queueService, catalog: catalogService,
		status: StatusStopped, volume: 1,
	}
}

// Play inicia la primera entrada pendiente. Si el Core fue reiniciado con una
// entrada persisted como playing, recupera y reproduce esa misma entrada.
func (s *Service) Play(ctx context.Context) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == StatusPlaying || s.status == StatusPaused {
		return State{}, ErrAlreadyPlaying
	}
	if err := s.playNextLocked(ctx); err != nil {
		return State{}, err
	}
	return s.stateLocked(), nil
}

func (s *Service) Pause() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshStatusLocked()
	if s.status != StatusPlaying {
		return State{}, ErrNotPlaying
	}
	if err := s.player.Pause(); err != nil {
		return State{}, fmt.Errorf("pausando reproducción: %w", err)
	}
	s.status = StatusPaused
	return s.stateLocked(), nil
}

func (s *Service) Resume() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != StatusPaused {
		return State{}, ErrNotPaused
	}
	if err := s.player.Resume(); err != nil {
		return State{}, fmt.Errorf("reanudando reproducción: %w", err)
	}
	s.status = StatusPlaying
	return s.stateLocked(), nil
}

func (s *Service) Stop(ctx context.Context) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.stopLocked(ctx, false); err != nil {
		return State{}, err
	}
	return s.stateLocked(), nil
}

// Next completa la entrada actual y reproduce la siguiente entrada FIFO.
func (s *Service) Next(ctx context.Context) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.stopLocked(ctx, true); err != nil {
		return State{}, err
	}
	if err := s.playNextLocked(ctx); errors.Is(err, queuecore.ErrNoNextSong) {
		// Avanzar desde la última canción deja al player detenido; no es un
		// fallo para la UI ni para el flujo normal de una rockola.
		return s.stateLocked(), nil
	} else if err != nil {
		return State{}, err
	}
	return s.stateLocked(), nil
}

func (s *Service) SetVolume(volume float64) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.player.SetVolume(volume); err != nil {
		return State{}, fmt.Errorf("ajustando volumen: %w", err)
	}
	s.volume = volume
	return s.stateLocked(), nil
}

func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateLocked()
}

func (s *Service) playNextLocked(ctx context.Context) error {
	entry, err := s.queue.GetCurrentSong(ctx)
	if errors.Is(err, queuecore.ErrNoCurrentSong) {
		entry, err = s.queue.GetNextSong(ctx)
		if err != nil {
			return err
		}
		if err := s.catalog.ValidateMediaPath(&entry.Song); err != nil {
			return err
		}
		if err := s.queue.MarkPlaying(ctx, entry.ID); err != nil {
			return err
		}
		entry.Status = queuecore.StatusPlaying
	} else if err != nil {
		return err
	} else if err := s.catalog.ValidateMediaPath(&entry.Song); err != nil {
		_ = s.queue.MarkCancelled(ctx, entry.ID)
		return err
	}

	if err := s.player.Play(entry.Song.FilePath); err != nil {
		_ = s.queue.MarkCancelled(ctx, entry.ID)
		return fmt.Errorf("reproduciendo canción: %w", err)
	}
	s.current = entry
	s.status = StatusPlaying
	return nil
}

func (s *Service) stopLocked(ctx context.Context, completed bool) error {
	s.refreshStatusLocked()
	if s.status != StatusEnded {
		if err := s.player.Stop(); err != nil {
			return fmt.Errorf("deteniendo reproducción: %w", err)
		}
	}
	entry := s.current
	if entry == nil {
		persisted, err := s.queue.GetCurrentSong(ctx)
		if err == nil {
			entry = persisted
		} else if !errors.Is(err, queuecore.ErrNoCurrentSong) {
			return err
		}
	}
	if entry != nil {
		var err error
		if completed {
			err = s.queue.MarkCompleted(ctx, entry.ID)
		} else {
			err = s.queue.MarkCancelled(ctx, entry.ID)
		}
		if err != nil {
			return err
		}
	}
	s.current = nil
	s.status = StatusStopped
	return nil
}

func (s *Service) stateLocked() State {
	s.refreshStatusLocked()
	return State{
		Status: s.status, QueueEntry: s.current,
		PositionMS: s.player.Position().Milliseconds(),
		DurationMS: s.player.Duration().Milliseconds(), Volume: s.volume,
	}
}

func (s *Service) refreshStatusLocked() {
	if s.status == StatusPlaying && !s.player.IsPlaying() {
		s.status = StatusEnded
	}
}
