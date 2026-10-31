package http

import (
	"errors"
	"net/http"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func handlerDeleteAd(deleteAd usecase.DeleteAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		userID := middleware.UserIDFromRequest(r)
		err = deleteAd(r.Context(), usecase.DeleteAdRequest{
			ID:          adID,
			RequesterID: userID,
		})

		switch {
		case err == nil:
			rw.WriteHeader(http.StatusNoContent)
		case errors.Is(err, domain.ErrAdNotFound):
			rw.WriteHeader(http.StatusNotFound)
		case errors.Is(err, domain.ErrNotOwner):
			rw.WriteHeader(http.StatusForbidden)
		default:
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}
}
