package providers

import (
	"context"

	"github.com/undeschimb/undeschimb/internal/domain"
)

type Provider interface {
	ID() string
	Fetch(ctx context.Context) ([]domain.RateSnapshot, error)
}

