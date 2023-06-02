package main

import (
	"fmt"
	"time"

	"github.com/zerodha/mii-lama/internal/metrics"
	"github.com/zerodha/mii-lama/internal/nse"
	"github.com/zerodha/mii-lama/pkg/models"
	"golang.org/x/exp/slog"
)

type App struct {
	lo   *slog.Logger
	opts Opts

	metricsMgr *metrics.Manager
	nseMgr     *nse.Manager

	hardwareSvc *hardwareService
}

type Opts struct {
	MaxRetries    int
	RetryInterval time.Duration
	SyncInterval  time.Duration
}

type hardwareService struct {
	hosts   []string
	queries map[string]string
}

// FetchMetrics fetches metrics from the Prometheus HTTP API.
func (app *App) FetchMetrics() (map[string]models.HWPromResp, error) {
	hwMetrics := make(map[string]models.HWPromResp)

	for _, host := range app.hardwareSvc.hosts {
		hwMetricsResp := models.HWPromResp{}
		for metric, query := range app.hardwareSvc.queries {
			switch metric {
			case "cpu":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host))
				if err != nil {
					app.lo.Error("querying prometheus failed", "error", err)
					continue
				}
				hwMetricsResp.CPU = value

			case "memory":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host, host, host, host))
				if err != nil {
					app.lo.Error("querying prometheus failed", "error", err)
					continue
				}
				hwMetricsResp.Mem = value

			case "disk":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host, host))
				if err != nil {
					app.lo.Error("querying prometheus failed", "error", err)
					continue
				}
				hwMetricsResp.Disk = value

			case "uptime":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host, host))
				if err != nil {
					app.lo.Error("querying prometheus failed", "error", err)
					continue
				}
				hwMetricsResp.Uptime = value

			default:
				app.lo.Warn("unknown metric: %s", metric)
			}
		}

		// Add host metrics to the map.
		hwMetrics[host] = hwMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "data", hwMetricsResp)
	}

	return hwMetrics, nil
}
