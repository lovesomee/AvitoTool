package metrics

import (
	"avitoproject/internal/client/avito"
	"context"
	"go.uber.org/zap"
)

type ServiceMetrics struct {
	logger     *zap.Logger
	repository Repository
}

func NewServiceMetrics(logger *zap.Logger, repository Repository) *ServiceMetrics {
	return &ServiceMetrics{
		logger:     logger,
		repository: repository,
	}
}

func (s *ServiceMetrics) UpdateSheet(ctx context.Context, shopName string, metrics avito.AvitoMetricsData) error {
	s.logger.Debug("updating google sheet", zap.String("service", "metrics"))

	if err := s.repository.UpdateGoogleSheet(ctx, shopName, metrics); err != nil {
		s.logger.Error("Failed to update sheet", zap.Error(err))
		return err
	}

	s.logger.Debug("google sheet updated", zap.String("service", "metrics"))
	return nil
}
