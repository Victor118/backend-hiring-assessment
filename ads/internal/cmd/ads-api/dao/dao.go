package dao

import (
	"context"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

// DAO is an AdRepository that can open transactions. Every call made directly on it is its own transaction.
type DAO interface {
	ports.AdRepository
	// Begin opens a transaction. Every call made on the returned Tx is part of it, and the caller must end it with Commit or Rollback.
	Begin(ctx context.Context) (Tx, error)
}

// Tx is an AdRepository scoped to an opened transaction: every call made on it is part of that transaction.
type Tx interface {
	ports.AdRepository
	// Commit applies every change made through the transaction.
	Commit() error
	// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
	Rollback() error
}
