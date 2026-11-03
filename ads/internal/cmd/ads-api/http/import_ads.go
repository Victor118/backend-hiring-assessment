package http

import (
	"encoding/json"
	"errors"
	"net/http"

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

func handlerImportAds(importAds usecase.ImportAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
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
			IdempotencyKey: "",
		}
		ids, err := importAds(r.Context(), importAdsReq)
		if err != nil {
			if errors.Is(err, domain.ErrInvalidAd) {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			rw.WriteHeader(http.StatusInternalServerError)
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
