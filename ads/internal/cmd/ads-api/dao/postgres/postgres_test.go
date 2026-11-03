package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

var adCols = []string{"id", "title", "price_cents", "photo_url", "user_id", "created_at"}

// arrayConverter lets []int64 arguments through, as pgx does: it encodes them as PostgreSQL arrays, whereas the default
// converter of sqlmock rejects them.
type arrayConverter struct{}

func (arrayConverter) ConvertValue(v any) (driver.Value, error) {
	if ids, ok := v.([]int64); ok {
		return ids, nil
	}

	return driver.DefaultParameterConverter.ConvertValue(v)
}

func newMockDAO(t *testing.T) (*DAO, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.ValueConverterOption(arrayConverter{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &DAO{db: db}, mock
}

func TestSearchAds(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns matching ads", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(1), "bike", int64(5000), "https://example.com/bike.jpg", "user-1", createdAt).
			AddRow(int64(2), "scooter", int64(12000), "https://example.com/scooter.jpg", "user-2", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnRows(rows)

		got, err := dao.SearchAds(ctx, ports.AdFilter{})

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.AdID(1), got[0].ID)
		assert.Equal(t, "bike", got[0].Title)
		assert.Equal(t, int64(5000), got[0].Price)
		assert.Equal(t, domain.AdID(2), got[1].ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns nil when no ads match", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "bike").
			WillReturnRows(sqlmock.NewRows(adCols))

		got, err := dao.SearchAds(ctx, ports.AdFilter{Title: "bike"})

		require.NoError(t, err)
		assert.Nil(t, got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when query fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnError(boom)

		_, err := dao.SearchAds(ctx, ports.AdFilter{})

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns ad on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(42), "mountain bike", int64(15000), "https://example.com/bike.jpg", "user-1", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(42)).
			WillReturnRows(rows)

		got, err := dao.GetAdByID(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(42), got.ID)
		assert.Equal(t, "mountain bike", got.Title)
		assert.Equal(t, int64(15000), got.Price)
		assert.Equal(t, domain.UserID("user-1"), got.UserID)
		assert.Equal(t, createdAt, got.CreatedAt)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrAdNotFound when no row", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(99)).
			WillReturnRows(sqlmock.NewRows(adCols))

		_, err := dao.GetAdByID(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAdNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on query failure", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("connection reset")
		mock.ExpectQuery("SELECT").
			WithArgs(int64(1)).
			WillReturnError(boom)

		_, err := dao.GetAdByID(ctx, 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDeleteAd(t *testing.T) {
	ctx := context.Background()
	adID := domain.AdID(1)

	t.Run("delete the ad", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectExec("DELETE FROM ads").
			WithArgs(int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := dao.DeleteAd(ctx, adID)

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when exec fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		dbDown := errors.New("db down")
		mock.ExpectExec("DELETE FROM ads").
			WithArgs(int64(1)).
			WillReturnError(dbDown)

		err := dao.DeleteAd(ctx, adID)

		require.Error(t, err)
		assert.ErrorIs(t, err, dbDown)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCreateAd(t *testing.T) {
	ctx := context.Background()
	req := domain.AdDraft{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("returns generated ID on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

		got, err := dao.CreateAd(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrUnknownOwner on FK violation", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		pgErr := &pgconn.PgError{Code: "23503"}
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(pgErr)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on other failures", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(boom)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, domain.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTransaction(t *testing.T) {
	ctx := context.Background()
	req := domain.AdDraft{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("commits every call made on the transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		first, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)
		second, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Commit())
		assert.Equal(t, domain.AdID(1), first)
		assert.Equal(t, domain.AdID(2), second)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back without committing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the transaction cannot be opened", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectBegin().WillReturnError(boom)

		tx, err := dao.Begin(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, tx)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollback after commit does nothing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rejects a nested transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, nestedErr := tx.(*DAO).Begin(ctx)

		require.Error(t, nestedErr)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("refuses commit and rollback outside a transaction", func(t *testing.T) {
		dao, _ := newMockDAO(t)

		require.Error(t, dao.Commit())
		require.Error(t, dao.Rollback())
	})
}

// newMockTx returns a DAO scoped to an opened transaction, for the methods that only make sense within one.
func newMockTx(t *testing.T) (*DAO, sqlmock.Sqlmock) {
	t.Helper()
	dao, mock := newMockDAO(t)
	mock.ExpectBegin()

	tx, err := dao.Begin(context.Background())
	require.NoError(t, err)

	return tx.(*DAO), mock
}

func TestReserveImport(t *testing.T) {
	ctx := context.Background()
	const (
		owner = domain.UserID("00000000-0000-0000-0000-000000000001")
		key   = "6f1c2b8e-3d4a-4f5b-9c6d-7e8f9a0b1c2d"
	)
	hash := []byte("fingerprint")

	t.Run("reserves a key that was never used", func(t *testing.T) {
		tx, mock := newMockTx(t)
		mock.ExpectExec("INSERT INTO import_requests").
			WithArgs(string(owner), key, hash).
			WillReturnResult(sqlmock.NewResult(0, 1))

		previous, found, err := tx.ReserveImport(ctx, owner, key, hash)

		require.NoError(t, err)
		assert.False(t, found)
		assert.Zero(t, previous)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns the stored import when the key was already used", func(t *testing.T) {
		tx, mock := newMockTx(t)
		mock.ExpectExec("INSERT INTO import_requests").
			WithArgs(string(owner), key, hash).
			WillReturnResult(sqlmock.NewResult(0, 0))
		// pgx hands arrays over to database/sql in their text format.
		mock.ExpectQuery("SELECT request_hash, ad_ids FROM import_requests").
			WithArgs(string(owner), key).
			WillReturnRows(sqlmock.NewRows([]string{"request_hash", "ad_ids"}).AddRow([]byte("stored"), "{10,11,12}"))

		previous, found, err := tx.ReserveImport(ctx, owner, key, hash)

		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, domain.ImportRecord{RequestHash: []byte("stored"), AdIDs: []domain.AdID{10, 11, 12}}, previous)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the key cannot be reserved", func(t *testing.T) {
		tx, mock := newMockTx(t)
		boom := errors.New("db down")
		mock.ExpectExec("INSERT INTO import_requests").
			WithArgs(string(owner), key, hash).
			WillReturnError(boom)

		_, found, err := tx.ReserveImport(ctx, owner, key, hash)

		require.ErrorIs(t, err, boom)
		assert.False(t, found)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the stored import cannot be read", func(t *testing.T) {
		tx, mock := newMockTx(t)
		boom := errors.New("db down")
		mock.ExpectExec("INSERT INTO import_requests").
			WithArgs(string(owner), key, hash).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("SELECT request_hash, ad_ids FROM import_requests").
			WithArgs(string(owner), key).
			WillReturnError(boom)

		_, found, err := tx.ReserveImport(ctx, owner, key, hash)

		require.ErrorIs(t, err, boom)
		assert.False(t, found)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("refuses to reserve outside a transaction, where the key would not lock anything", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		_, _, err := dao.ReserveImport(ctx, owner, key, hash)

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCompleteImport(t *testing.T) {
	ctx := context.Background()
	const (
		owner = domain.UserID("00000000-0000-0000-0000-000000000001")
		key   = "6f1c2b8e-3d4a-4f5b-9c6d-7e8f9a0b1c2d"
	)

	t.Run("stores the IDs under the reserved key", func(t *testing.T) {
		tx, mock := newMockTx(t)
		mock.ExpectExec("UPDATE import_requests SET ad_ids").
			WithArgs(string(owner), key, []int64{10, 11, 12}).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := tx.CompleteImport(ctx, owner, key, []domain.AdID{10, 11, 12})

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the key was not reserved", func(t *testing.T) {
		tx, mock := newMockTx(t)
		mock.ExpectExec("UPDATE import_requests SET ad_ids").
			WithArgs(string(owner), key, []int64{10}).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := tx.CompleteImport(ctx, owner, key, []domain.AdID{10})

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on failure", func(t *testing.T) {
		tx, mock := newMockTx(t)
		boom := errors.New("db down")
		mock.ExpectExec("UPDATE import_requests SET ad_ids").
			WithArgs(string(owner), key, []int64{10}).
			WillReturnError(boom)

		err := tx.CompleteImport(ctx, owner, key, []domain.AdID{10})

		require.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("refuses to complete outside a transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		err := dao.CompleteImport(ctx, owner, key, []domain.AdID{10})

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
