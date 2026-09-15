package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"rockola/core/credits"
)

type CreditRepository struct {
	db *sql.DB
}

var _ credits.Repository = (*CreditRepository)(nil)

func NewCreditRepository(db *sql.DB) *CreditRepository {
	return &CreditRepository{db: db}
}

func (r *CreditRepository) GetBalance(ctx context.Context) (credits.Balance, error) {
	return getBalance(ctx, r.db)
}

func (r *CreditRepository) AddCredits(ctx context.Context, amount int) (credits.Balance, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return credits.Balance{}, fmt.Errorf("iniciando transacción: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx,
		`UPDATE credits SET balance = balance + ?, updated_at = ? WHERE id = 1`,
		amount, formatTime(now)); err != nil {
		return credits.Balance{}, fmt.Errorf("actualizando saldo: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_transactions (amount, credits, type, created_at)
		VALUES (?, ?, 'add', ?)`, amount, amount, formatTime(now)); err != nil {
		return credits.Balance{}, fmt.Errorf("registrando transacción: %w", err)
	}
	balance, err := getBalance(ctx, tx)
	if err != nil {
		return credits.Balance{}, err
	}
	if err := tx.Commit(); err != nil {
		return credits.Balance{}, fmt.Errorf("confirmando créditos: %w", err)
	}
	return balance, nil
}

func (r *CreditRepository) ConsumeCredit(ctx context.Context) (credits.Balance, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return credits.Balance{}, fmt.Errorf("iniciando transacción: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE credits SET balance = balance - 1, updated_at = ?
		WHERE id = 1 AND balance > 0`, formatTime(now))
	if err != nil {
		return credits.Balance{}, fmt.Errorf("actualizando saldo: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return credits.Balance{}, fmt.Errorf("verificando saldo: %w", err)
	}
	if affected == 0 {
		return credits.Balance{}, credits.ErrInsufficientCredit
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_transactions (amount, credits, type, created_at)
		VALUES (0, -1, 'consume', ?)`, formatTime(now)); err != nil {
		return credits.Balance{}, fmt.Errorf("registrando consumo: %w", err)
	}
	balance, err := getBalance(ctx, tx)
	if err != nil {
		return credits.Balance{}, err
	}
	if err := tx.Commit(); err != nil {
		return credits.Balance{}, fmt.Errorf("confirmando consumo: %w", err)
	}
	return balance, nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getBalance(ctx context.Context, q queryRower) (credits.Balance, error) {
	var balance credits.Balance
	var updatedAt string
	if err := q.QueryRowContext(ctx,
		`SELECT balance, updated_at FROM credits WHERE id = 1`).Scan(
		&balance.Credits, &updatedAt); err != nil {
		return credits.Balance{}, fmt.Errorf("consultando saldo: %w", err)
	}
	balance.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return balance, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
