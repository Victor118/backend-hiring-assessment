package http

import (
	"bytes"
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

// importKey is a valid Idempotency-Key, in its canonical lowercase form.
const importKey = "6f1c2b8e-3d4a-4f5b-9c6d-7e8f9a0b1c2d"

// postImport posts an import with the given Authorization and Idempotency-Key headers, each left out when empty.
func postImport(t *testing.T, url string, body any, authHeader, idempotencyKey string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
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

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", importKey)
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

		resp := postImport(t, srv.URL+"/ads/import", body, "user-42", importKey)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, domain.UserID("user-42"), received.OwnerID)
		assert.Equal(t, importKey, received.IdempotencyKey)
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

		resp := postImport(t, srv.URL+"/ads/import", importAdsBody{Ads: []createAdBody{}}, "user-1", importKey)
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

		resp := postImport(t, srv.URL+"/ads/import", body, "", importKey)
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
		// A valid key, so that the 400 comes from the missing body.
		req.Header.Set("Idempotency-Key", importKey)

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

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", importKey)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("returns 400 when an ad is invalid", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, fmt.Errorf("unable to import ad 2: %w", domain.ErrInvalidAd)
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", importKey)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 when Idempotency-Key is missing", func(t *testing.T) {
		var called bool
		srv := newImportAdsServer(recordCall(&called))
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.False(t, called, "usecase should not be called")
	})

	t.Run("returns 400 when Idempotency-Key is not a UUID", func(t *testing.T) {
		var called bool
		srv := newImportAdsServer(recordCall(&called))
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", "not-a-uuid")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.False(t, called, "usecase should not be called")
	})

	t.Run("passes the Idempotency-Key in its canonical form", func(t *testing.T) {
		var received usecase.ImportAdsRequest
		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{10, 11, 12}, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", "6F1C2B8E-3D4A-4F5B-9C6D-7E8F9A0B1C2D")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		// The same key in upper case is the same key: it must not let the import run twice.
		assert.Equal(t, importKey, received.IdempotencyKey)
	})

	t.Run("returns 422 when the key was already used for another content", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, fmt.Errorf("unable to import ads: %w", domain.ErrImportKeyReused)
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", importKey)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}
