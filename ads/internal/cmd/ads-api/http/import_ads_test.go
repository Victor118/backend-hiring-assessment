package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newImportAdsServer(fn usecase.ImportAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(fn)))
	return httptest.NewServer(mux)
}

// recordCall returns an ImportAdsFunc that only records that it was called. The usecase runs on the server
// goroutine, where t.Fatal must not be used, so the test asserts on the flag once the response is received.
func recordCall(called *bool) usecase.ImportAdsFunc {
	return func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
		*called = true
		return nil, nil
	}
}

func TestHandlerImportAds(t *testing.T) {
	body := importAdsBody{Ads: []createAdBody{
		{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
		{Title: "scooter", PriceCents: 8000, PhotoURL: "https://example.com/scooter.jpg"},
		{Title: "car", PriceCents: 200000, PhotoURL: "https://example.com/car.jpg"},
	}}

	t.Run("returns 201 with the IDs of the imported ads", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return []domain.AdID{10, 11, 12}, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{10, 11, 12}, got.IDs)
	})

	t.Run("passes the ads and the authenticated user as owner to the usecase", func(t *testing.T) {
		var received usecase.ImportAdsRequest
		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{10, 11, 12}, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-42")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, domain.UserID("user-42"), received.OwnerID)
		assert.Equal(t, []usecase.ImportedAd{
			{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
			{Title: "scooter", PriceCents: 8000, PhotoURL: "https://example.com/scooter.jpg"},
			{Title: "car", PriceCents: 200000, PhotoURL: "https://example.com/car.jpg"},
		}, received.Ads)
	})

	t.Run("returns 201 with an empty list of IDs when body has no ads", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return []domain.AdID{}, nil
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", importAdsBody{Ads: []createAdBody{}}, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// An empty array, not null: clients iterate over the IDs without checking for null.
		assert.JSONEq(t, `{"ids":[]}`, string(raw))
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		var called bool
		srv := newImportAdsServer(recordCall(&called))
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", body, "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.False(t, called, "usecase should not be called")
	})

	t.Run("returns 400 on invalid body", func(t *testing.T) {
		var called bool
		srv := newImportAdsServer(recordCall(&called))
		defer srv.Close()

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads/import", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "user-1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.False(t, called, "usecase should not be called")
	})

	t.Run("returns 500 when the import fails", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, errors.New("db down")
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("returns 400 when an ad is invalid", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, fmt.Errorf("unable to import ad 2: %w", domain.ErrInvalidAd)
		})
		defer srv.Close()

		resp := postJSON(t, srv.URL+"/ads/import", body, "user-1")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
