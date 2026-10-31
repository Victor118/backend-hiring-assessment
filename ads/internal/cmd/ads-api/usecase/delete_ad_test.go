package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestDeleteAd(t *testing.T) {
	ctx := context.Background()
	adID := domain.AdID(1)
	deleteRequest := usecase.DeleteAdRequest{
		ID: adID,
	}

	t.Run("nominal", func(t *testing.T) {
		//given
		dao := mockdao.New(t)
		dao.On("GetAdByID", ctx, deleteRequest.ID).Return(domain.Ad{ID: 1}, nil)
		dao.On("DeleteAd", ctx, adID).Return(nil)
		deleteAd := usecase.NewDeleteAdFunc(dao)
		//when
		err := deleteAd(ctx, deleteRequest)
		//then
		require.NoError(t, err)
	})

	t.Run("ad not found", func(t *testing.T) {
		//given
		dao := mockdao.New(t)
		dao.On("GetAdByID", ctx, domain.AdID(1)).Return(domain.Ad{}, domain.ErrAdNotFound)

		deleteAd := usecase.NewDeleteAdFunc(dao)
		//when
		err := deleteAd(ctx, deleteRequest)
		//then
		dao.AssertNotCalled(t, "DeleteAd", mock.Anything, mock.Anything)
		require.ErrorIs(t, err, domain.ErrAdNotFound)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		//given
		dao := mockdao.New(t)
		dao.On("GetAdByID", ctx, deleteRequest.ID).Return(domain.Ad{ID: 1}, nil)
		dbDown := errors.New("db down")
		dao.On("DeleteAd", ctx, adID).Return(dbDown)
		deleteAd := usecase.NewDeleteAdFunc(dao)
		//when
		err := deleteAd(ctx, deleteRequest)
		//then
		require.ErrorIs(t, err, dbDown)
	})
}
