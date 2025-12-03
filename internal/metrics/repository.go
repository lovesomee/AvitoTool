package metrics

import (
	"avitoproject/internal/client/avito"
	"context"
)

type Repository interface {
	UpdateGoogleSheet(ctx context.Context, shopName string, data avito.AvitoMetricsData) error
}
