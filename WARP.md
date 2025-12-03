# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Key commands

This is a Go module (`go.mod` at the repo root). All commands below are intended to be run from the repository root.

- **Build all packages**
  - `go build ./...`
- **Run the main application** (starts schedulers and blocks forever)
  - `go run ./main`
- **Run all tests** (when tests are added)
  - `go test ./...`
- **Run tests in a single package**
  - `go test ./internal/metrics`
- **Run a single test by name** (example)
  - `go test ./internal/metrics -run TestName`

### Configuration required to run

The application will not start correctly without the following local files in the repository root:

- `service_account.json` — Google service account JSON key used by the Google Sheets client.
- `config/config.json` — configuration file read via Viper with the following high-level structure:
  - `shops[]` — list of shops; each shop includes:
    - `name` — logical shop name.
    - `clientId`, `clientSecret` — Avito API credentials per shop.
    - `userId` — Avito user ID for metrics.
    - `sheetRange` — A1 range in the target Google Sheet for current metrics.
    - `snapshots[]` — list of snapshot definitions with `time` (HH:MM, MSK) and `range` (A1 range for snapshot writes).
  - `urls` — Avito endpoints:
    - `tokenUrl` — OAuth token endpoint.
    - `metricsUrl` — base URL for metrics (per-user path is appended).
  - `sheetId` — Google Sheets spreadsheet ID.

## Architecture overview

### High-level data flow

1. **Configuration & logging**
   - `config.Read()` loads `config/config.json` into `config.Config`.
   - A production `zap.Logger` instance is created in `main/main.go` and passed down into all major components.
2. **External clients**
   - **Google Sheets client** (`internal/client/google`):
     - `NewGoogleClient("service_account.json", cfg.SheetId)` builds a Sheets API client using a service account and target spreadsheet ID.
     - Exposes methods to read/write ranges and perform batch updates: `UpdateSheet`, `BatchUpdate`, `ReadRange`.
   - **Avito client** (`internal/client/avito`):
     - `NewAvitoClient(logger, cfg.Urls.TokenUrl, cfg.Urls.MetricsUrl)` wraps Avito OAuth and metrics APIs.
     - Maintains an in-memory token cache per `client_id` with expiry (`tokenCache`), refreshing tokens as needed.
     - Provides:
       - `GetAvitoMetrics` — fetches aggregate (totals) metrics for a user for the current day.
       - `GetMetricsForAllItems` — fetches active items, their per-item metrics, and bid data, then computes derived per-item values.
3. **Domain & persistence layer (metrics)**
   - **Interface** (`internal/metrics/repository.go`):
     - `Repository` defines `UpdateGoogleSheet(ctx, shopName, data AvitoMetricsData) error` as the abstraction for writing aggregate metrics.
   - **Google Sheets-backed repository** (`internal/metrics/repository_gs.go`):
     - `RepositoryMetrics` implements `Repository` and also additional snapshot-related methods.
     - Resolves the correct `sheetRange` for a shop from `config.Config` and writes metrics into the target ranges via the Google client.
     - Snapshot-related responsibilities:
       - `SaveSnapshotsIfDue` — on each tick, checks configured snapshot times per shop (in MSK) and, when due, reads the current range and writes a transposed copy into configured snapshot ranges via batch updates.
       - `ClearAllSnapshotRanges` — clears all configured snapshot ranges (called daily at 00:00 MSK).
       - `UpdateGoogleSheetForItemsHourly` — writes per-item metrics into the configured range for a given shop.
   - **Service layer** (`internal/metrics/service.go`):
     - `ServiceMetrics` holds a `Repository` and a logger.
     - `UpdateSheet` is a thin domain-level wrapper that logs around repository writes for aggregate metrics.
4. **Worker orchestration** (`internal/worker/worker.go`)
   - `Worker` coordinates Avito metrics retrieval and persistence for all shops defined in the config.
   - `ProcessAllShops(ctx)` loop:
     - Iterates over `cfg.Shops`.
     - For each shop, calls `avito.GetAvitoMetrics` with that shop's `UserId`, `ClientId`, and `ClientSecret`.
     - Delegates persistence to `service.UpdateSheet`, which ultimately writes to Google Sheets.
     - Sleeps `65 * time.Second` between shops to avoid rate limiting.

5. **Scheduling & background jobs** (`internal/cron`)
   - **Main scheduler** (`cron.go`):
     - `Scheduler` wraps a `robfig/cron` instance, a `Worker`, and a logger.
     - `Start(ctx)`:
       - Immediately kicks off `worker.ProcessAllShops` in a goroutine.
       - Schedules `ProcessAllShops` every 10 minutes using a cron spec with seconds enabled.
   - **Snapshot scheduler** (`cron_snapshots.go`):
     - `SnapshotScheduler` wraps a cron instance, a `*metrics.RepositoryMetrics` (concrete repository), and a logger.
     - `Start(ctx)` schedules two jobs:
       - Every minute: `repo.SaveSnapshotsIfDue(ctx)` — takes snapshots of current metrics into configured snapshot ranges.
       - Every day at `00:00` MSK: `repo.ClearAllSnapshotRanges(ctx)` — clears all snapshot ranges.

6. **Application entrypoint** (`main/main.go`)
   - Creates the global `zapLogger` and base `context.Background()`.
   - Loads configuration with `config.Read()`.
   - Instantiates the Google Sheets client and metrics repository/service.
   - Instantiates the Avito client and `Worker`.
   - Creates and starts:
     - `Scheduler` (metrics update worker every 10 minutes, plus initial run).
     - `SnapshotScheduler` (minute-level snapshot checks and nightly cleanup).
   - Uses `select {}` at the end of `main` to block indefinitely while both schedulers run.

### Architectural notes for future changes

- Keep the separation between:
  - **External API clients** (`internal/client/...`),
  - **Domain/services** (`internal/metrics`), and
  - **Orchestration/scheduling** (`internal/worker`, `internal/cron`).
- Prefer adding new Avito or Sheets functionality via the existing client packages, and surface them through the metrics repository/service rather than calling external APIs directly from cron or worker code.
- If new types of persistence or output targets are added, implement additional `Repository` implementations and inject them into `ServiceMetrics` rather than bypassing the interface.
