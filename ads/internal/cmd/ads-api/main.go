package main

import (
	"context"
	"log/slog"
	"os"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/http"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/identity/userapi"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/httpserver"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx := context.Background()
	cfg := Load()

	adDAO, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := adDAO.Close(); err != nil {
			slog.Error("unable to close the database", "error", err)
		}
	}()

	usersClient := api.NewClient(cfg.IdentityURL, cfg.IdentityTimeout)
	identities := userapi.New(usersClient)

	router := http.NewRouter(http.Usecases{
		CreateAd: usecase.NewCreateAdFunc(adDAO),
		GetAd:    usecase.NewGetAdByIDFunc(adDAO, identities, logger),
		ListAds:  usecase.NewListAdsFunc(adDAO),
		DeleteAd: usecase.NewDeleteAdFunc(adDAO),
	})

	return httpserver.Run(ctx, cfg.HTTPAddr, router)
}
