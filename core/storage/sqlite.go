// Package storage implementa la persistencia en SQLite.
// Es la única capa que conoce SQL; el dominio (core/catalog, etc.)
// interactúa a través de interfaces.
package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // driver SQLite puro Go (sin CGO)

	"rockola/database"
)

// Open abre (o crea) la base de datos SQLite en `path` y aplica el esquema.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	// busy_timeout evita errores "database is locked" con accesos concurrentes.
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abriendo sqlite %s: %w", path, err)
	}
	// El driver puro Go serializa mejor con una sola conexión de escritura.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("conectando a sqlite %s: %w", path, err)
	}

	if _, err := db.ExecContext(ctx, database.Schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("aplicando esquema: %w", err)
	}
	return db, nil
}
