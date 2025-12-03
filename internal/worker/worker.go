package worker

import (
	"avitoproject/config"
	"avitoproject/internal/client/avito"
	"avitoproject/internal/metrics"
	"context"
	"go.uber.org/zap"
	"time"
)

type Worker struct {
	logger  *zap.Logger
	avito   *avito.AvitoClient
	service *metrics.ServiceMetrics
	cfg     config.Config
}

func NewWorker(
	logger *zap.Logger,
	avitoClient *avito.AvitoClient,
	service *metrics.ServiceMetrics,
	cfg config.Config,
) *Worker {
	return &Worker{
		logger:  logger,
		avito:   avitoClient,
		service: service,
		cfg:     cfg,
	}
}

func (w *Worker) ProcessAllShops(ctx context.Context) {
	for _, shop := range w.cfg.Shops {
		w.logger.Info("Processing shop", zap.String("name", shop.Name))

		metrics, err := w.avito.GetAvitoMetrics(shop.UserId, shop.ClientId, shop.ClientSecret)
		if err != nil {
			w.logger.Error("Failed to get metrics", zap.String("shop", shop.Name), zap.Error(err))
			continue
		}

		w.logger.Info("Successfully retrieved metrics", zap.String("shop", shop.Name), zap.Any("metrics", metrics))

		if err := w.service.UpdateSheet(ctx, shop.Name, metrics); err != nil {
			w.logger.Error("Failed to update sheet", zap.String("shop", shop.Name), zap.Error(err))
		}

		w.logger.Info("Successfully updated sheet", zap.String("shop", shop.Name))

		time.Sleep(65 * time.Second)
	}
}
