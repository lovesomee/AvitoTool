package main

import (
	"avitoproject/config"
	"avitoproject/internal/client/avito"
	googleClient "avitoproject/internal/client/google"
	"avitoproject/internal/cron"
	"avitoproject/internal/metrics"
	"avitoproject/internal/worker"
	"context"
	"log"

	"go.uber.org/zap"
)

func main() {
	// Логгер
	zapLogger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer zapLogger.Sync()
	ctx := context.Background()

	// Конфиг
	cfg := config.Read()

	// Google клиент
	gClient, err := googleClient.NewGoogleClient("service_account.json", cfg.SheetId)
	if err != nil {
		zapLogger.Fatal("failed to create Google client", zap.Error(err))
	}

	// Репозиторий и сервис
	repo := metrics.NewRepositoryMetrics(zapLogger, cfg, gClient)
	service := metrics.NewServiceMetrics(zapLogger, repo)

	// Avito клиент
	avitoClient := avito.NewAvitoClient(zapLogger, cfg.Urls.TokenUrl, cfg.Urls.MetricsUrl)

	// Worker
	w := worker.NewWorker(zapLogger, avitoClient, service, cfg)

	// Cron scheduler
	s := cron.NewScheduler(zapLogger, w)
	if err = s.Start(ctx); err != nil {
		zapLogger.Fatal("failed to start cron scheduler", zap.Error(err))
	}
	defer s.Stop()

	snapshotCron := cron.NewSnapshotScheduler(zapLogger, repo)
	snapshotCron.Start(ctx)
	defer snapshotCron.Stop()

	select {}
}
