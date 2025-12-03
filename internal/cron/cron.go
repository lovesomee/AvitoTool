package cron

import (
	"avitoproject/internal/worker"
	"context"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Scheduler struct {
	cron   *cron.Cron
	worker *worker.Worker
	logger *zap.Logger
}

func NewScheduler(logger *zap.Logger, w *worker.Worker) *Scheduler {
	c := cron.New(cron.WithSeconds()) // включаем секунды для гибкости
	return &Scheduler{
		cron:   c,
		worker: w,
		logger: logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	// Запускаем воркер сразу при старте
	s.logger.Info("Initial run of Avito metrics update")
	go s.worker.ProcessAllShops(ctx)

	// Планируем повторный запуск каждые 10 минут
	_, err := s.cron.AddFunc("@every 10m", func() {
		s.logger.Info("Scheduled Avito metrics update")
		s.worker.ProcessAllShops(ctx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	s.logger.Info("Cron scheduler started")
	return nil
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.logger.Info("Cron scheduler stopped")
}
