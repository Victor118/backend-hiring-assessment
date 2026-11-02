package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

var _ ports.Identities = (*Mock)(nil)

// Mock is a testify-based implementation of ports.Identities.
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

// GetUsername returns the username and the error set on the mock.
func (m *Mock) GetUsername(ctx context.Context, userID domain.UserID) (string, error) {
	args := m.Called(ctx, userID)

	username, _ := args.Get(0).(string)

	return username, args.Error(1)
}
