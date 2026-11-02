package userapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/identity/userapi"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

func setupSrv(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/users/{id}", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func TestGetUsername(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		srv := setupSrv(t, func(rw http.ResponseWriter, r *http.Request) {
			if r.PathValue("id") != "1" {
				rw.WriteHeader(http.StatusNotFound)

				return
			}

			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"user_id":"1","nickname":"alice"}`))
		})
		identities := userapi.New(api.NewClient(srv.URL, time.Second))

		username, err := identities.GetUsername(ctx, domain.UserID("1"))

		require.NoError(t, err)
		assert.Equal(t, "alice", username)
	})

	t.Run("returns ErrUserNotFound when the users service answers 404", func(t *testing.T) {
		srv := setupSrv(t, func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
		})
		identities := userapi.New(api.NewClient(srv.URL, time.Second))

		_, err := identities.GetUsername(ctx, domain.UserID("9999"))

		require.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("returns an error other than ErrUserNotFound when the users service answers 500", func(t *testing.T) {
		srv := setupSrv(t, func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
		})
		identities := userapi.New(api.NewClient(srv.URL, time.Second))

		_, err := identities.GetUsername(ctx, domain.UserID("9999"))

		require.Error(t, err)
		assert.NotErrorIs(t, err, domain.ErrUserNotFound)
	})
}
