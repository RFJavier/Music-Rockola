package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rockola/api"
	"rockola/core/catalog"
	"rockola/core/credits"
	"rockola/core/playback"
	queuecore "rockola/core/queue"
	"rockola/core/storage"
)

func TestCatalogCreditsAndQueueAPI(t *testing.T) {
	handler, db, _ := testHandler(t)

	response := request(t, handler, http.MethodGet, "/api/songs", nil)
	assertStatus(t, response, http.StatusOK)
	var empty []catalog.Song
	decode(t, response, &empty)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("catálogo vacío = %#v", empty)
	}

	musicDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(musicDir, "Soda Stereo - Uno.mp3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(musicDir, "Soda Stereo - Dos.wav"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	response = request(t, handler, http.MethodPost, "/api/catalog/scan", map[string]any{"path": musicDir})
	assertStatus(t, response, http.StatusOK)
	var scan catalog.ScanResult
	decode(t, response, &scan)
	if scan.Added != 2 {
		t.Fatalf("scan.Added = %d", scan.Added)
	}

	response = request(t, handler, http.MethodGet, "/api/songs/search?q=Soda", nil)
	assertStatus(t, response, http.StatusOK)
	var songs []catalog.Song
	decode(t, response, &songs)
	if len(songs) != 2 {
		t.Fatalf("resultados = %d", len(songs))
	}

	response = request(t, handler, http.MethodPost, "/api/queue", map[string]any{"song_id": songs[0].ID})
	assertError(t, response, http.StatusConflict, "insufficient_credits")
	response = request(t, handler, http.MethodPost, "/api/credits/add", map[string]any{"amount": 2})
	assertStatus(t, response, http.StatusOK)

	for _, song := range songs {
		response = request(t, handler, http.MethodPost, "/api/queue", map[string]any{"song_id": song.ID})
		assertStatus(t, response, http.StatusCreated)
	}
	response = request(t, handler, http.MethodGet, "/api/credits", nil)
	assertStatus(t, response, http.StatusOK)
	var balance credits.Balance
	decode(t, response, &balance)
	if balance.Credits != 0 {
		t.Fatalf("balance = %d", balance.Credits)
	}

	response = request(t, handler, http.MethodGet, "/api/queue", nil)
	assertStatus(t, response, http.StatusOK)
	var entries []queuecore.Entry
	decode(t, response, &entries)
	if len(entries) != 2 || entries[0].Position != 1 || entries[1].Position != 2 {
		t.Fatalf("cola no FIFO: %+v", entries)
	}

	// El consumo fallido no deja una transacción de auditoría huérfana.
	var transactionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credit_transactions`).Scan(&transactionCount); err != nil {
		t.Fatal(err)
	}
	if transactionCount != 3 { // alta + dos consumos exitosos
		t.Fatalf("transacciones = %d", transactionCount)
	}
}

func TestPlayerAPIAdvancesQueue(t *testing.T) {
	handler, _, fake := testHandler(t)
	musicDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(musicDir, "Primera.wav"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(musicDir, "Segunda.wav"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	response := request(t, handler, http.MethodPost, "/api/catalog/scan", map[string]any{"path": musicDir})
	assertStatus(t, response, http.StatusOK)
	response = request(t, handler, http.MethodPost, "/api/credits/add", map[string]any{"amount": 2})
	assertStatus(t, response, http.StatusOK)
	response = request(t, handler, http.MethodPost, "/api/queue", map[string]any{"song_id": 1})
	assertStatus(t, response, http.StatusCreated)
	response = request(t, handler, http.MethodPost, "/api/queue", map[string]any{"song_id": 2})
	assertStatus(t, response, http.StatusCreated)

	response = request(t, handler, http.MethodPost, "/api/player/play", nil)
	assertStatus(t, response, http.StatusOK)
	var state playback.State
	decode(t, response, &state)
	if state.Status != playback.StatusPlaying || state.QueueEntry == nil || state.QueueEntry.Position != 1 {
		t.Fatalf("estado inicial = %+v", state)
	}

	response = request(t, handler, http.MethodPost, "/api/player/pause", nil)
	assertStatus(t, response, http.StatusOK)
	response = request(t, handler, http.MethodPost, "/api/player/resume", nil)
	assertStatus(t, response, http.StatusOK)
	response = request(t, handler, http.MethodPost, "/api/player/next", nil)
	assertStatus(t, response, http.StatusOK)
	decode(t, response, &state)
	if state.QueueEntry == nil || state.QueueEntry.Position != 2 || fake.playCalls != 2 {
		t.Fatalf("estado después de next = %+v, plays=%d", state, fake.playCalls)
	}

	response = request(t, handler, http.MethodPost, "/api/player/volume", map[string]any{"volume": 0.4})
	assertStatus(t, response, http.StatusOK)
	response = request(t, handler, http.MethodPost, "/api/player/stop", nil)
	assertStatus(t, response, http.StatusOK)
	decode(t, response, &state)
	if state.Status != playback.StatusStopped {
		t.Fatalf("estado detenido = %+v", state)
	}
}

func TestAPIValidation(t *testing.T) {
	handler, _, _ := testHandler(t)
	tests := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/songs/search", nil},
		{http.MethodPost, "/api/credits/add", map[string]any{"amount": 0}},
		{http.MethodPost, "/api/queue", map[string]any{"song_id": -1}},
		{http.MethodPost, "/api/player/volume", map[string]any{"volume": 2}},
		{http.MethodDelete, "/api/queue/no-es-id", nil},
	}
	for _, test := range tests {
		response := request(t, handler, test.method, test.path, test.body)
		assertError(t, response, http.StatusBadRequest, "invalid_request")
	}
}

func TestPlayerNaturalEndIsNotAnMCIError(t *testing.T) {
	handler, _, fake := testHandler(t)
	musicDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(musicDir, "Unica.wav"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, request(t, handler, http.MethodPost, "/api/catalog/scan",
		map[string]any{"path": musicDir}), http.StatusOK)
	assertStatus(t, request(t, handler, http.MethodPost, "/api/credits/add",
		map[string]any{"amount": 1}), http.StatusOK)
	assertStatus(t, request(t, handler, http.MethodPost, "/api/queue",
		map[string]any{"song_id": 1}), http.StatusCreated)
	assertStatus(t, request(t, handler, http.MethodPost, "/api/player/play", nil), http.StatusOK)

	// Simula que MCI llegó al final antes de la siguiente petición HTTP.
	fake.playing = false
	response := request(t, handler, http.MethodGet, "/api/player", nil)
	assertStatus(t, response, http.StatusOK)
	var state playback.State
	decode(t, response, &state)
	if state.Status != playback.StatusEnded {
		t.Fatalf("status = %s, want ended", state.Status)
	}

	response = request(t, handler, http.MethodPost, "/api/player/pause", nil)
	assertError(t, response, http.StatusConflict, "invalid_state")
	response = request(t, handler, http.MethodPost, "/api/player/next", nil)
	assertStatus(t, response, http.StatusOK)
	decode(t, response, &state)
	if state.Status != playback.StatusStopped {
		t.Fatalf("status después de next sin cola = %s", state.Status)
	}
	assertStatus(t, request(t, handler, http.MethodPost, "/api/player/stop", nil), http.StatusOK)
}

func testHandler(t *testing.T) (http.Handler, *sql.DB, *fakePlayer) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "rockola.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	songRepo := storage.NewSongRepository(db)
	catalogService := catalog.NewService(songRepo, logger)
	queueService := queuecore.NewService(storage.NewQueueRepository(db), catalogService)
	creditService := credits.NewService(storage.NewCreditRepository(db))
	fake := &fakePlayer{}
	playbackService := playback.NewService(fake, queueService, catalogService)
	server := api.NewServer(
		catalogService, catalog.NewScanner(songRepo, logger), queueService,
		creditService, playbackService, logger,
	)
	return server.Handler(), db, fake
}

type fakePlayer struct {
	playing   bool
	paused    bool
	volume    float64
	playCalls int
}

func (p *fakePlayer) Play(string) error {
	p.playing = true
	p.paused = false
	p.playCalls++
	return nil
}
func (p *fakePlayer) Pause() error {
	p.playing, p.paused = false, true
	return nil
}
func (p *fakePlayer) Resume() error {
	p.playing, p.paused = true, false
	return nil
}
func (p *fakePlayer) Stop() error {
	p.playing, p.paused = false, false
	return nil
}
func (p *fakePlayer) SetVolume(volume float64) error { p.volume = volume; return nil }
func (p *fakePlayer) IsPlaying() bool                { return p.playing }
func (p *fakePlayer) Position() time.Duration        { return time.Second }
func (p *fakePlayer) Duration() time.Duration        { return 3 * time.Minute }

func request(t *testing.T, handler http.Handler, method, path string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	handler.ServeHTTP(recorder, req)
	return recorder.Result()
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("status = %d, want %d, body=%s", response.StatusCode, want, data)
	}
}

func assertError(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	assertStatus(t, response, status)
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode(t, response, &payload)
	if payload.Error.Code != code {
		t.Fatalf("error.code = %q, want %q", payload.Error.Code, code)
	}
}

func decode(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}
