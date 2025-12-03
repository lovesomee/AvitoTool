package metrics

import (
	"avitoproject/config"
	"avitoproject/internal/client/avito"
	googleClient "avitoproject/internal/client/google"
	"context"
	"fmt"
	"go.uber.org/zap"
	"time"
)

type RepositoryMetrics struct {
	logger *zap.Logger
	cfg    config.Config
	client *googleClient.Client
}

func NewRepositoryMetrics(logger *zap.Logger, cfg config.Config, client *googleClient.Client) *RepositoryMetrics {
	return &RepositoryMetrics{
		logger: logger,
		cfg:    cfg,
		client: client,
	}
}

func (r *RepositoryMetrics) UpdateGoogleSheet(ctx context.Context, shopName string, data avito.AvitoMetricsData) error {
	var writeRange string
	for _, shop := range r.cfg.Shops {
		if shop.Name == shopName {
			writeRange = shop.SheetRange
			break
		}
	}
	if writeRange == "" {
		return fmt.Errorf("sheet range not found for shop %s", shopName)
	}

	// подготовка значений
	values := [][]interface{}{
		{data.Spending / 100},
		{data.Impressions},
		{data.Views},
		{data.Contacts},
	}

	// вызов метода из Google клиента
	if err := r.client.UpdateSheet(writeRange, values); err != nil {
		return fmt.Errorf("unable to write data to sheet: %w", err)
	}

	r.logger.Debug("successfully wrote data to sheet", zap.String("shop", shopName))
	return nil
}

func (r *RepositoryMetrics) SaveSnapshotsIfDue(ctx context.Context) {
	msk := time.FixedZone("MSK", 3*3600)
	now := time.Now().In(msk)
	current := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	updates := make(map[string][][]interface{})

	for _, shop := range r.cfg.Shops {
		for _, snap := range shop.Snapshots {
			if snap.Time == current {
				data, err := r.client.ReadRange(shop.SheetRange)
				if err != nil {
					r.logger.Error("Failed to read current range", zap.String("shop", shop.Name), zap.Error(err))
					continue
				}

				// Транспонируем данные, чтобы строки стали колонками
				updates[snap.Range] = transpose(data)
			}
		}
	}

	if len(updates) > 0 {
		if err := r.client.BatchUpdate(updates); err != nil {
			r.logger.Error("Failed to write snapshots", zap.Error(err))
		} else {
			r.logger.Info("Snapshots saved", zap.Any("ranges", updates))
		}
	}
}

// Вспомогательная функция транспонирования
func transpose(values [][]interface{}) [][]interface{} {
	if len(values) == 0 {
		return [][]interface{}{}
	}

	n, m := len(values), len(values[0])
	result := make([][]interface{}, m)
	for i := range result {
		result[i] = make([]interface{}, n)
		for j := range values {
			if i < len(values[j]) {
				result[i][j] = values[j][i]
			} else {
				result[i][j] = ""
			}
		}
	}
	return result
}

func (r *RepositoryMetrics) ClearAllSnapshotRanges(ctx context.Context) error {
	updates := make(map[string][][]interface{})

	for _, shop := range r.cfg.Shops {
		for _, snap := range shop.Snapshots {
			// Полная очистка диапазона
			updates[snap.Range] = [][]interface{}{}
		}
	}

	if len(updates) == 0 {
		return nil
	}

	if err := r.client.BatchUpdate(updates); err != nil {
		return fmt.Errorf("failed to clear snapshot ranges: %w", err)
	}

	r.logger.Info("All snapshot ranges cleared at 00:00")
	return nil
}

func (r *RepositoryMetrics) UpdateGoogleSheetForItemsHourly(ctx context.Context, items []avito.ItemMetrics, shop config.Shop, logger *zap.Logger) error {
	logger.Info("Start UpdateGoogleSheetForItemsHourly", zap.String("shop", shop.Name), zap.Int("itemsCount", len(items)))

	values := [][]interface{}{}

	// текущий час в MSK
	now := time.Now().In(time.FixedZone("MSK", 3*3600))
	hour := now.Hour()

	for _, it := range items {
		row := []interface{}{
			it.Link,
			it.Title,
			it.ID,
			it.Impressions,
			it.Views,
			it.Contacts,
			it.Spending / 100,
			0, 0, // conversion metrics пока 0
			it.CostPerContactLastHour,
			0, 0, 0, // diffs
			it.BidPenny,
		}
		values = append(values, row)
		logger.Debug("Prepared row for sheet", zap.Int64("itemID", it.ID), zap.Int("hour", hour))
	}

	if err := r.client.UpdateSheet(shop.SheetRange, values); err != nil {
		logger.Error("Failed to update Google Sheet", zap.String("range", shop.SheetRange), zap.Error(err))
		return err
	}

	logger.Info("Google Sheet updated successfully", zap.String("range", shop.SheetRange), zap.Int("rowsWritten", len(values)))
	return nil
}
