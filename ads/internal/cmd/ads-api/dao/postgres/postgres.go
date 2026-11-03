package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

const adColumns = `id, title, price_cents, photo_url, user_id::text, created_at`

var (
	_ ports.TransactionalAdRepository = (*DAO)(nil)
	_ ports.AdTx                      = (*DAO)(nil)
)

// DAO is a PostgreSQL-backed ad store.
type DAO struct {
	db *sql.DB
	// tx is set when the DAO is scoped to a transaction opened by Begin.
	tx *sql.Tx
}

// queryRunner is the subset of *sql.DB and *sql.Tx the queries need.
type queryRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// New opens a connection pool to the given DSN and checks that the database is reachable. The caller is responsible for calling Close.
func New(ctx context.Context, dsn string) (*DAO, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("unable to reach database: %w", err)
	}

	return &DAO{db: db}, nil
}

// Close releases the connection pool.
func (d *DAO) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("unable to close database: %w", err)
	}

	return nil
}

// runner returns the transaction the DAO is scoped to, or the connection pool.
func (d *DAO) runner() queryRunner {
	if d.tx != nil {
		return d.tx
	}

	return d.db
}

// Begin opens a transaction. Every call made on the returned Tx is part of it, and the caller must end it with Commit or Rollback.
func (d *DAO) Begin(ctx context.Context) (ports.AdTx, error) {
	if d.tx != nil {
		return nil, errors.New("unable to open a nested transaction")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	return &DAO{db: d.db, tx: tx}, nil
}

// ReserveImport claims the import key for the owner within the transaction. The primary key of import_requests is
// what serializes concurrent imports: a second transaction inserting the same key waits until the first one ends,
// then either finds the committed import or, if it was rolled back, reserves the key itself.
func (d *DAO) ReserveImport(ctx context.Context, ownerID domain.UserID, key string, requestHash []byte) (domain.ImportRecord, bool, error) {
	if d.tx == nil {
		return domain.ImportRecord{}, false, errors.New("unable to reserve the import outside a transaction")
	}

	const reserve = `
		INSERT INTO import_requests (user_id, idempotency_key, request_hash)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING`

	result, err := d.tx.ExecContext(ctx, reserve, string(ownerID), key, requestHash)
	if err != nil {
		return domain.ImportRecord{}, false, fmt.Errorf("unable to reserve the import: %w", err)
	}

	reserved, err := result.RowsAffected()
	if err != nil {
		return domain.ImportRecord{}, false, fmt.Errorf("unable to reserve the import: %w", err)
	}

	if reserved == 1 {
		return domain.ImportRecord{}, false, nil
	}

	const stored = `
		SELECT request_hash, ad_ids FROM import_requests
		WHERE user_id = $1::uuid AND idempotency_key = $2::uuid`

	var (
		previous domain.ImportRecord
		adIDs    []int64
	)
	// database/sql cannot scan a PostgreSQL array: the pgtype scanner decodes the text format pgx hands over.
	err = d.tx.QueryRowContext(ctx, stored, string(ownerID), key).Scan(&previous.RequestHash, pgtype.NewMap().SQLScanner(&adIDs))
	if err != nil {
		return domain.ImportRecord{}, false, fmt.Errorf("unable to read the stored import: %w", err)
	}

	previous.AdIDs = make([]domain.AdID, 0, len(adIDs))
	for _, adID := range adIDs {
		previous.AdIDs = append(previous.AdIDs, domain.AdID(adID))
	}

	return previous, true, nil
}

// CompleteImport stores the identifiers of the ads created under an import key reserved within the transaction.
func (d *DAO) CompleteImport(ctx context.Context, ownerID domain.UserID, key string, adIDs []domain.AdID) error {
	if d.tx == nil {
		return errors.New("unable to complete the import outside a transaction")
	}

	const query = `
		UPDATE import_requests SET ad_ids = $3
		WHERE user_id = $1::uuid AND idempotency_key = $2::uuid`

	ids := make([]int64, 0, len(adIDs))
	for _, adID := range adIDs {
		ids = append(ids, int64(adID))
	}

	result, err := d.tx.ExecContext(ctx, query, string(ownerID), key, ids)
	if err != nil {
		return fmt.Errorf("unable to complete the import: %w", err)
	}

	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to complete the import: %w", err)
	}

	if updated != 1 {
		return fmt.Errorf("unable to complete the import: key %s is not reserved", key)
	}

	return nil
}

// Commit applies every change made through the transaction.
func (d *DAO) Commit() error {
	if d.tx == nil {
		return errors.New("unable to commit outside a transaction")
	}

	if err := d.tx.Commit(); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}

	return nil
}

// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
func (d *DAO) Rollback() error {
	if d.tx == nil {
		return errors.New("unable to roll back outside a transaction")
	}

	if err := d.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("unable to roll back transaction: %w", err)
	}

	return nil
}

// SearchAds returns all ads matching the filters.
func (d *DAO) SearchAds(ctx context.Context, filter ports.AdFilter) ([]domain.Ad, error) {
	const query = `
		SELECT ` + adColumns + `
		FROM ads
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = 0 OR price_cents >= $2)
		  AND ($3 = 0 OR price_cents <= $3)
		  AND ($4 = '' OR title ILIKE '%' || $4 || '%')`

	rows, err := d.runner().QueryContext(ctx, query,
		filter.OwnerID,
		filter.MinPriceCents,
		filter.MaxPriceCents,
		filter.Title,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to search ads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ads []domain.Ad
	for rows.Next() {
		var ad domain.Ad
		if err := rows.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
			return nil, fmt.Errorf("unable to scan ad: %w", err)
		}

		ads = append(ads, ad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unable to read ads: %w", err)
	}

	return ads, nil
}

// GetAdByID returns a single ad, or domain.ErrAdNotFound.
func (d *DAO) GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	const query = `SELECT ` + adColumns + ` FROM ads WHERE id = $1`

	ad, err := scanAd(d.runner().QueryRowContext(ctx, query, int64(id)))
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get ad: %w", err)
	}

	return ad, nil
}

// CreateAd inserts an ad and returns its generated identifier.
func (d *DAO) CreateAd(ctx context.Context, ad domain.AdDraft) (domain.AdID, error) {
	const query = `
		INSERT INTO ads (title, price_cents, photo_url, user_id)
		VALUES ($1, $2, $3, $4::uuid)
		RETURNING id`

	var id domain.AdID
	err := d.runner().QueryRowContext(ctx, query, ad.Title, ad.PriceCents, ad.PhotoURL, string(ad.OwnerID)).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return 0, fmt.Errorf("unable to create ad: %w", domain.ErrUnknownOwner)
		}

		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return id, nil
}

// DeleteAd deletes a single ad.
func (d *DAO) DeleteAd(ctx context.Context, id domain.AdID) error {
	const query = `DELETE FROM ads WHERE id = $1`

	_, err := d.runner().ExecContext(ctx, query, int64(id))
	if err != nil {
		return fmt.Errorf("unable to delete ad: %w", err)
	}

	return nil
}

func scanAd(row *sql.Row) (domain.Ad, error) {
	var ad domain.Ad
	if err := row.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Ad{}, domain.ErrAdNotFound
		}

		return domain.Ad{}, err
	}

	return ad, nil
}
