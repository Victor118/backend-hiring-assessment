package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type importAdsBody struct {
	Ads []createAdBody `json:"ads"`
}

type importAdsResponse struct {
	IDs []int64 `json:"ids"`
}

// handlerImportAds serves POST /ads/import. The Idempotency-Key header, a UUID, is required: retrying an import with
// the same key answers with the IDs of the ads it created instead of creating them twice.
func handlerImportAds(importAds usecase.ImportAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		// Parsed rather than passed through: an invalid key is a client error, and the canonical form makes the same
		// key in upper case the same key.
		key, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		var body importAdsBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		ownerID := domain.UserID(middleware.UserIDFromRequest(r))
		ads := make([]usecase.ImportedAd, 0, len(body.Ads))
		for _, item := range body.Ads {
			importedAd := usecase.ImportedAd{
				Title:      item.Title,
				PriceCents: item.PriceCents,
				PhotoURL:   item.PhotoURL,
			}

			ads = append(ads, importedAd)
		}
		importAdsReq := usecase.ImportAdsRequest{
			OwnerID:        ownerID,
			Ads:            ads,
			IdempotencyKey: key.String(),
		}
		ids, err := importAds(r.Context(), importAdsReq)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrInvalidAd):
				rw.WriteHeader(http.StatusBadRequest)
			case errors.Is(err, domain.ErrImportKeyReused):
				rw.WriteHeader(http.StatusUnprocessableEntity)
			default:
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(adIDsToImportAdsResponse(ids))
	}
}

// adIDsToImportAdsResponse converts the IDs one by one: Go does not convert a []domain.AdID into a []int64, even though
// AdID is an int64. The slice is never nil, so that no ads are encoded as [] rather than null.
func adIDsToImportAdsResponse(adIDs []domain.AdID) importAdsResponse {
	ids := make([]int64, 0, len(adIDs))
	for _, adID := range adIDs {
		ids = append(ids, int64(adID))
	}

	return importAdsResponse{IDs: ids}
}
