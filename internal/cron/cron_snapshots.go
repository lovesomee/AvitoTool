package cron

import (
	"avitoproject/internal/metrics"
	"context"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type SnapshotScheduler struct {
	cron   *cron.Cron
	repo   *metrics.RepositoryMetrics
	logger *zap.Logger
}

func NewSnapshotScheduler(logger *zap.Logger, repo *metrics.RepositoryMetrics) *SnapshotScheduler {
	c := cron.New(cron.WithSeconds())
	return &SnapshotScheduler{
		cron:   c,
		repo:   repo,
		logger: logger,
	}
}

func (s *SnapshotScheduler) Start(ctx context.Context) {

	// --- 1. Каждую минуту — проверка и сохранение снапшотов ---
	_, err := s.cron.AddFunc("0 * * * * *", func() {
		s.repo.SaveSnapshotsIfDue(ctx)
	})
	if err != nil {
		s.logger.Fatal("failed to add snapshot cron func", zap.Error(err))
	}

	// --- 2. Каждый день в 00:00 — очистка всех snapshot-диапазонов ---
	_, err = s.cron.AddFunc("0 0 0 * * *", func() {
		s.logger.Info("running daily cleanup of all snapshot ranges")

		if err := s.repo.ClearAllSnapshotRanges(ctx); err != nil {
			s.logger.Error("failed to clear snapshot ranges", zap.Error(err))
		} else {
			s.logger.Info("all snapshot ranges cleared successfully")
		}
	})
	if err != nil {
		s.logger.Fatal("failed to add daily clear cron func", zap.Error(err))
	}

	s.cron.Start()
	s.logger.Info("Snapshot cron started")
}

func (s *SnapshotScheduler) Stop() {
	s.cron.Stop()
	s.logger.Info("Snapshot cron stopped")
}
