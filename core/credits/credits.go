// Package credits contiene las reglas del saldo simulado de la rockola.
package credits

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidAmount      = errors.New("la cantidad de créditos debe ser mayor que cero")
	ErrInsufficientCredit = errors.New("créditos insuficientes")
)

// Balance representa el saldo actual persistido.
type Balance struct {
	Credits   int       `json:"balance"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Transaction conserva la auditoría de cada alta o consumo.
type Transaction struct {
	ID        int64     `json:"id"`
	Amount    int       `json:"amount"`
	Credits   int       `json:"credits"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository abstrae la persistencia de créditos.
type Repository interface {
	GetBalance(ctx context.Context) (Balance, error)
	AddCredits(ctx context.Context, amount int) (Balance, error)
	ConsumeCredit(ctx context.Context) (Balance, error)
}

// Service aplica las reglas del saldo simulado.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetBalance(ctx context.Context) (Balance, error) {
	return s.repo.GetBalance(ctx)
}

// AddCredits agrega créditos administrativos. Para este MVP, amount expresa
// tanto la cantidad simulada como los créditos otorgados.
func (s *Service) AddCredits(ctx context.Context, amount int) (Balance, error) {
	if amount <= 0 {
		return Balance{}, ErrInvalidAmount
	}
	balance, err := s.repo.AddCredits(ctx, amount)
	if err != nil {
		return Balance{}, fmt.Errorf("agregando créditos: %w", err)
	}
	return balance, nil
}

func (s *Service) ConsumeCredit(ctx context.Context) (Balance, error) {
	balance, err := s.repo.ConsumeCredit(ctx)
	if err != nil {
		return Balance{}, fmt.Errorf("consumiendo crédito: %w", err)
	}
	return balance, nil
}
