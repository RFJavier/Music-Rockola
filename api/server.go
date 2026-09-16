// Package api expone los servicios del Core mediante HTTP/JSON local.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rockola/core/catalog"
	"rockola/core/credits"
	"rockola/core/playback"
	queuecore "rockola/core/queue"
)

type Server struct {
	catalog  *catalog.Service
	scanner  *catalog.Scanner
	queue    *queuecore.Service
	credits  *credits.Service
	playback *playback.Service
	log      *slog.Logger
}

func NewServer(
	catalogService *catalog.Service,
	scanner *catalog.Scanner,
	queueService *queuecore.Service,
	creditService *credits.Service,
	playbackService *playback.Service,
	logger *slog.Logger,
) *Server {
	return &Server{
		catalog: catalogService, scanner: scanner, queue: queueService,
		credits: creditService, playback: playbackService, log: logger,
	}
}

// Handler devuelve un http.Handler listo para usar por el servidor y tests.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/songs", s.listSongs)
	mux.HandleFunc("GET /api/songs/search", s.searchSongs)
	mux.HandleFunc("GET /api/songs/{id}", s.getSong)
	mux.HandleFunc("GET /api/artists", s.listArtists)
	mux.HandleFunc("GET /api/albums", s.listAlbums)
	mux.HandleFunc("GET /api/genres", s.listGenres)
	mux.HandleFunc("POST /api/catalog/scan", s.scanMusic)

	mux.HandleFunc("GET /api/queue", s.getQueue)
	mux.HandleFunc("POST /api/queue", s.addQueue)
	mux.HandleFunc("DELETE /api/queue", s.clearQueue)
	mux.HandleFunc("DELETE /api/queue/{id}", s.removeQueue)

	mux.HandleFunc("GET /api/credits", s.getCredits)
	mux.HandleFunc("POST /api/credits/add", s.addCredits)
	mux.HandleFunc("POST /api/credits/consume", s.consumeCredit)

	mux.HandleFunc("GET /api/player", s.getPlayer)
	mux.HandleFunc("POST /api/player/play", s.play)
	mux.HandleFunc("POST /api/player/pause", s.pause)
	mux.HandleFunc("POST /api/player/resume", s.resume)
	mux.HandleFunc("POST /api/player/stop", s.stop)
	mux.HandleFunc("POST /api/player/next", s.next)
	mux.HandleFunc("POST /api/player/volume", s.setVolume)

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return s.logging(mux)
}

func (s *Server) listSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := s.catalog.ListSongs(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	if songs == nil {
		songs = []catalog.Song{}
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) searchSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := s.catalog.SearchSongs(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if songs == nil {
		songs = []catalog.Song{}
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) getSong(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	song, err := s.catalog.GetSong(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.catalog.ValidateMediaPath(song); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, song)
}

func (s *Server) listArtists(w http.ResponseWriter, r *http.Request) {
	s.listStrings(w, r, s.catalog.ListArtists)
}

func (s *Server) listAlbums(w http.ResponseWriter, r *http.Request) {
	s.listStrings(w, r, s.catalog.ListAlbums)
}

func (s *Server) listGenres(w http.ResponseWriter, r *http.Request) {
	s.listStrings(w, r, s.catalog.ListGenres)
}

func (s *Server) listStrings(w http.ResponseWriter, r *http.Request, fn func(context.Context) ([]string, error)) {
	values, err := fn(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	if values == nil {
		values = []string{}
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) scanMusic(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &request); err != nil {
		s.writeError(w, err)
		return
	}
	if strings.TrimSpace(request.Path) == "" {
		s.writeError(w, badRequest("path es obligatorio"))
		return
	}
	if strings.Contains(request.Path, "\x00") {
		s.writeError(w, badRequest("path inválido: contiene carácter nulo"))
		return
	}
	result, err := s.scanner.ScanMusicFolder(r.Context(), request.Path)
	if err != nil {
		s.writeError(w, badRequest(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getQueue(w http.ResponseWriter, r *http.Request) {
	entries, err := s.queue.GetQueue(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	if entries == nil {
		entries = []queuecore.Entry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) addQueue(w http.ResponseWriter, r *http.Request) {
	var request struct {
		SongID int64 `json:"song_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		s.writeError(w, err)
		return
	}
	if request.SongID <= 0 {
		s.writeError(w, badRequest("song_id debe ser mayor que cero"))
		return
	}
	entry, err := s.queue.AddToQueue(r.Context(), request.SongID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) removeQueue(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.queue.RemoveFromQueue(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) clearQueue(w http.ResponseWriter, r *http.Request) {
	// Detener primero evita que MCI siga reproduciendo una entrada que acaba de
	// ser cancelada por ClearQueue.
	if _, err := s.playback.Stop(r.Context()); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.queue.ClearQueue(r.Context()); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getCredits(w http.ResponseWriter, r *http.Request) {
	balance, err := s.credits.GetBalance(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, balance)
}

func (s *Server) addCredits(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Amount int `json:"amount"`
	}
	if err := decodeJSON(r, &request); err != nil {
		s.writeError(w, err)
		return
	}
	balance, err := s.credits.AddCredits(r.Context(), request.Amount)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, balance)
}

func (s *Server) consumeCredit(w http.ResponseWriter, r *http.Request) {
	balance, err := s.credits.ConsumeCredit(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, balance)
}

func (s *Server) getPlayer(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.playback.State())
}

func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	state, err := s.playback.Play(r.Context())
	s.writePlayerResult(w, state, err)
}

func (s *Server) pause(w http.ResponseWriter, _ *http.Request) {
	state, err := s.playback.Pause()
	s.writePlayerResult(w, state, err)
}

func (s *Server) resume(w http.ResponseWriter, _ *http.Request) {
	state, err := s.playback.Resume()
	s.writePlayerResult(w, state, err)
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	state, err := s.playback.Stop(r.Context())
	s.writePlayerResult(w, state, err)
}

func (s *Server) next(w http.ResponseWriter, r *http.Request) {
	state, err := s.playback.Next(r.Context())
	s.writePlayerResult(w, state, err)
}

func (s *Server) setVolume(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Volume float64 `json:"volume"`
	}
	if err := decodeJSON(r, &request); err != nil {
		s.writeError(w, err)
		return
	}
	if request.Volume < 0 || request.Volume > 1 {
		s.writeError(w, badRequest("volume debe estar entre 0 y 1"))
		return
	}
	state, err := s.playback.SetVolume(request.Volume)
	s.writePlayerResult(w, state, err)
}

func (s *Server) writePlayerResult(w http.ResponseWriter, state playback.State, err error) {
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

type requestError struct {
	message string
}

func (e requestError) Error() string  { return e.message }
func badRequest(message string) error { return requestError{message: message} }

func (s *Server) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	switch {
	case errors.As(err, new(requestError)), errors.Is(err, catalog.ErrEmptyQuery), errors.Is(err, credits.ErrInvalidAmount):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, catalog.ErrSongNotFound), errors.Is(err, queuecore.ErrEntryNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, catalog.ErrMediaNotFound):
		status, code = http.StatusUnprocessableEntity, "media_not_found"
	case errors.Is(err, credits.ErrInsufficientCredit):
		status, code = http.StatusConflict, "insufficient_credits"
	case errors.Is(err, queuecore.ErrNoCurrentSong), errors.Is(err, queuecore.ErrNoNextSong),
		errors.Is(err, queuecore.ErrInvalidTransition), errors.Is(err, playback.ErrAlreadyPlaying),
		errors.Is(err, playback.ErrNotPlaying), errors.Is(err, playback.ErrNotPaused):
		status, code = http.StatusConflict, "invalid_state"
	}
	if status == http.StatusInternalServerError {
		s.log.Error("error de API", "error", err)
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": err.Error()},
	})
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return badRequest("JSON inválido: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return badRequest("el body debe contener un único objeto JSON")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("id inválido")
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.log.Debug("petición HTTP", "method", r.Method, "path", r.URL.Path,
			"duration", time.Since(started))
	})
}
