package storage_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rockola/core/catalog"
	"rockola/core/credits"
	queuecore "rockola/core/queue"
	"rockola/core/storage"
)

func TestCreditsAndQueueFlow(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "rockola.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	songRepo := storage.NewSongRepository(db)
	mediaDir := t.TempDir()
	song1 := insertTestSong(t, ctx, songRepo, mediaDir, "Primera.mp3")
	song2 := insertTestSong(t, ctx, songRepo, mediaDir, "Segunda.mp3")

	creditService := credits.NewService(storage.NewCreditRepository(db))
	catalogService := catalog.NewService(songRepo, slog.Default())
	queueService := queuecore.NewService(storage.NewQueueRepository(db), catalogService)

	// Sin crédito no se crea cola ni transacción de consumo.
	if _, err := queueService.AddToQueue(ctx, song1.ID); !errors.Is(err, credits.ErrInsufficientCredit) {
		t.Fatalf("AddToQueue sin saldo: error = %v", err)
	}
	assertCount(t, db, "queue", 0)
	assertCount(t, db, "credit_transactions", 0)

	balance, err := creditService.AddCredits(ctx, 2)
	if err != nil || balance.Credits != 2 {
		t.Fatalf("AddCredits: balance = %+v, error = %v", balance, err)
	}
	first, err := queueService.AddToQueue(ctx, song1.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := queueService.AddToQueue(ctx, song2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Position != 1 || second.Position != 2 {
		t.Fatalf("posiciones no FIFO: %d, %d", first.Position, second.Position)
	}
	balance, err = creditService.GetBalance(ctx)
	if err != nil || balance.Credits != 0 {
		t.Fatalf("saldo final = %+v, error = %v", balance, err)
	}
	assertCount(t, db, "credit_transactions", 3) // alta + 2 consumos

	next, err := queueService.GetNextSong(ctx)
	if err != nil || next.ID != first.ID {
		t.Fatalf("siguiente FIFO = %+v, error = %v", next, err)
	}
	if err := queueService.MarkPlaying(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	current, err := queueService.GetCurrentSong(ctx)
	if err != nil || current.ID != first.ID || current.StartedAt == nil {
		t.Fatalf("actual = %+v, error = %v", current, err)
	}
	if err := queueService.MarkPlaying(ctx, second.ID); !errors.Is(err, queuecore.ErrInvalidTransition) {
		t.Fatalf("se permitió un segundo playing: %v", err)
	}
	if err := queueService.MarkCompleted(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	next, err = queueService.GetNextSong(ctx)
	if err != nil || next.ID != second.ID {
		t.Fatalf("siguiente después de completar = %+v, error = %v", next, err)
	}
	if err := queueService.RemoveFromQueue(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	active, err := queueService.GetQueue(ctx)
	if err != nil || len(active) != 0 {
		t.Fatalf("cola activa = %+v, error = %v", active, err)
	}

	var completed, cancelled int
	if err := db.QueryRow(`SELECT
		SUM(status = 'completed'), SUM(status = 'cancelled') FROM queue`).Scan(
		&completed, &cancelled); err != nil {
		t.Fatal(err)
	}
	if completed != 1 || cancelled != 1 {
		t.Fatalf("historial: completed=%d cancelled=%d", completed, cancelled)
	}
}

func TestMissingMediaDoesNotConsumeCredit(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "rockola.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	songRepo := storage.NewSongRepository(db)
	now := time.Now().UTC()
	song := &catalog.Song{
		Title: "No existe", FilePath: filepath.Join(t.TempDir(), "missing.mp3"),
		Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := songRepo.Insert(ctx, song); err != nil {
		t.Fatal(err)
	}
	creditService := credits.NewService(storage.NewCreditRepository(db))
	if _, err := creditService.AddCredits(ctx, 1); err != nil {
		t.Fatal(err)
	}
	queueService := queuecore.NewService(
		storage.NewQueueRepository(db), catalog.NewService(songRepo, slog.Default()))
	if _, err := queueService.AddToQueue(ctx, song.ID); !errors.Is(err, catalog.ErrMediaNotFound) {
		t.Fatalf("error esperado ErrMediaNotFound, recibido %v", err)
	}
	balance, err := creditService.GetBalance(ctx)
	if err != nil || balance.Credits != 1 {
		t.Fatalf("se consumió crédito con media faltante: %+v, %v", balance, err)
	}
}

func insertTestSong(t *testing.T, ctx context.Context, repo *storage.SongRepository, dir, name string) *catalog.Song {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	song := &catalog.Song{
		Title: name, FilePath: path, Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Insert(ctx, song); err != nil {
		t.Fatal(err)
	}
	return song
}

func assertCount(t *testing.T, db interface {
	QueryRow(query string, args ...any) *sql.Row
}, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}
