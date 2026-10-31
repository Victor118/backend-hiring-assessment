package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newDeleteAdServer(fn usecase.DeleteAdFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("DELETE /ads/{adID}", middleware.RequireAuth(handlerDeleteAd(fn)))
	return httptest.NewServer(mux)
}

func deleteRequest(t *testing.T, url, authHeader string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestHandlerDeleteAd(t *testing.T) {
	t.Run("returns 204 when the ad is deleted", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ usecase.DeleteAdRequest) error {
			return nil
		})
		defer srv.Close()

		resp := deleteRequest(t, srv.URL+"/ads/1", "bob")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	})

	t.Run("passes the ad ID and the requester to the usecase", func(t *testing.T) {
		var reqReceived usecase.DeleteAdRequest
		srv := newDeleteAdServer(func(_ context.Context, req usecase.DeleteAdRequest) error {
			reqReceived = req
			return nil
		})
		defer srv.Close()
		resp := deleteRequest(t, srv.URL+"/ads/7", "bob")
		defer resp.Body.Close()
		assert.Equal(t, domain.AdID(7), reqReceived.ID)
		assert.Equal(t, domain.UserID("bob"), reqReceived.RequesterID)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("returns 400 when adID is not a number", func(t *testing.T) {
		called := false
		srv := newDeleteAdServer(func(_ context.Context, req usecase.DeleteAdRequest) error {
			called = true
			return nil
		})
		defer srv.Close()
		resp := deleteRequest(t, srv.URL+"/ads/XXX", "bob")
		defer resp.Body.Close()
		assert.False(t, called, "usecase should not be called")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 401 when no authorization header", func(t *testing.T) {
		called := false
		srv := newDeleteAdServer(func(_ context.Context, req usecase.DeleteAdRequest) error {
			called = true
			return nil
		})
		defer srv.Close()
		resp := deleteRequest(t, srv.URL+"/ads/7", "")
		defer resp.Body.Close()
		assert.False(t, called, "usecase should not be called")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 404 when ad does not exist", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ usecase.DeleteAdRequest) error {

			return fmt.Errorf("unable to get ad 7: %w", domain.ErrAdNotFound)
		})
		defer srv.Close()
		resp := deleteRequest(t, srv.URL+"/ads/7", "bob")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns 403 when the requester is not the owner", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ usecase.DeleteAdRequest) error {

			return fmt.Errorf("unable to delete ad 7 by bob: %w", domain.ErrNotOwner)
		})
		defer srv.Close()
		resp := deleteRequest(t, srv.URL+"/ads/7", "bob")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("returns 500 on unexpected usecase error", func(t *testing.T) {
		srv := newDeleteAdServer(func(_ context.Context, _ usecase.DeleteAdRequest) error {

			return errors.New("db connection lost")
		})
		defer srv.Close()
		resp := deleteRequest(t, srv.URL+"/ads/7", "bob")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
