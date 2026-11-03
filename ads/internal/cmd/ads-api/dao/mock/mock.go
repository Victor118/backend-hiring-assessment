package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

var (
	_ ports.TransactionalAdRepository = (*Mock)(nil)
	_ ports.AdTx                      = (*Mock)(nil)
)

// Mock is a testify-based implementation of ports.TransactionalAdRepository.
type Mock struct {
	mock.Mock
}

// New returns a mock bound to the given test. Every expectation set on it is verified when the test finishes.
func New(t *testing.T) *Mock {
	t.Helper()

	m := &Mock{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })

	return m
}

// SearchAds returns the ads and the error set on the mock.
func (m *Mock) SearchAds(ctx context.Context, filter ports.AdFilter) ([]domain.Ad, error) {
	args := m.Called(ctx, filter)

	ads, _ := args.Get(0).([]domain.Ad)

	return ads, args.Error(1)
}

// GetAdByID returns the ad and the error set on the mock.
func (m *Mock) GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	args := m.Called(ctx, id)

	ad, _ := args.Get(0).(domain.Ad)

	return ad, args.Error(1)
}

// CreateAd returns the ad and the error set on the mock.
func (m *Mock) CreateAd(ctx context.Context, ad ports.NewAd) (domain.AdID, error) {
	args := m.Called(ctx, ad)

	adID, _ := args.Get(0).(domain.AdID)

	return adID, args.Error(1)
}

// DeleteAd returns the error set on the mock.
func (m *Mock) DeleteAd(ctx context.Context, id domain.AdID) error {
	return m.Called(ctx, id).Error(0)
}

// Begin returns the transaction and the error set on the mock, for example On("Begin", ctx).Return(tx, nil). Using another
// Mock as the transaction keeps the calls made inside it apart from the calls made on the repository itself.
func (m *Mock) Begin(ctx context.Context) (ports.AdTx, error) {
	args := m.Called(ctx)

	tx, _ := args.Get(0).(ports.AdTx)

	return tx, args.Error(1)
}

// Commit returns the error set on the mock.
func (m *Mock) Commit() error {
	return m.Called().Error(0)
}

// Rollback returns the error set on the mock.
func (m *Mock) Rollback() error {
	return m.Called().Error(0)
}
