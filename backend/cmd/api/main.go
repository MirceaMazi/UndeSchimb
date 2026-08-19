package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/undeschimb/undeschimb/internal/config"
	"github.com/undeschimb/undeschimb/internal/httpapi"
	"github.com/undeschimb/undeschimb/internal/providers"
	"github.com/undeschimb/undeschimb/internal/service"
	"github.com/undeschimb/undeschimb/internal/store"
)

func main() {
	configuration := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	appContext, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	database, err := store.Open(appContext, configuration.DatabaseURL)
	if err != nil {
		logger.Error("database is unavailable", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	if err := database.Migrate(appContext, configuration.MigrationsPath); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: configuration.HTTPTimeout}
	collector := service.NewCollector(database, logger, configuration.RefreshEvery,
		providers.NewBNRProvider(httpClient),
		providers.NewBancaTransilvaniaProvider(httpClient),
		providers.NewBCRProvider(httpClient),
		providers.NewBankPageProvider("brd", "https://www.brd.ro/curs-valutar-si-dobanzi-de-referinta", []string{"Schimb valutar în cont", "Account Exchange Rates"}, httpClient),
		providers.NewINGProvider(httpClient),
		providers.NewRaiffeisenProvider(httpClient),
		providers.NewCECProvider(httpClient),
		providers.NewXTBProvider(httpClient, configuration.XTBHolidays),
		providers.NewRevolutProvider(httpClient),
		providers.NewTavexProvider(httpClient),
		providers.NewLuxorBucharestProvider(httpClient),
	)
	go collector.Run(appContext)

	comparison := service.NewComparisonService(database, configuration.XTBHolidays)
	server := &http.Server{
		Addr:              ":" + configuration.Port,
		Handler:           httpapi.NewRouter(comparison, configuration.CorsOrigin),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-appContext.Done()
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownContext)
	}()
	logger.Info("UndeSchimb API is listening", "port", configuration.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
