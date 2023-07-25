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
	dbSvc       *dbService
	networkSvc  *networkService
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

type dbService struct {
	hosts   []string
	queries map[string]string
}

type networkService struct {
	hosts   []string
	queries map[string]string
}

// fetchHWMetrics fetches hardware metrics from the Prometheus HTTP API.
func (app *App) fetchHWMetrics() (map[string]models.HWPromResp, error) {
	hwMetrics := make(map[string]models.HWPromResp)

	for _, host := range app.hardwareSvc.hosts {
		hwMetricsResp := models.HWPromResp{}
		for metric, query := range app.hardwareSvc.queries {
			switch metric {
			case "cpu":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host))
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				hwMetricsResp.CPU = value

			case "memory":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host, host, host, host))
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				hwMetricsResp.Mem = value

			case "disk":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host, host))
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				hwMetricsResp.Disk = value

			case "uptime":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host, host))
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				hwMetricsResp.Uptime = value

			default:
				app.lo.Warn("Unknown hardware metric queried",
					"host", host,
					"metric", metric)
			}
		}

		// Add host metrics to the map.
		hwMetrics[host] = hwMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "data", hwMetricsResp)
	}

	return hwMetrics, nil
}

// fetchDBMetrics fetches database metrics from the Prometheus HTTP API.
func (app *App) fetchDBMetrics() (map[string]models.DBPromResp, error) {
	dbMetrics := make(map[string]models.DBPromResp)

	// Query database metrics.
	for _, host := range app.dbSvc.hosts {
		dbMetricsResp := models.DBPromResp{}
		for metric, query := range app.dbSvc.queries {
			switch metric {
			case "status":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host))
				if err != nil {
					app.lo.Error("Failed to query Prometheus for database status",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				dbMetricsResp.Status = value

			default:
				app.lo.Warn("Unknown database metric queried",
					"host", host,
					"metric", metric)
			}
		}

		// Add host metrics to the map.
		dbMetrics[host] = dbMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "data", dbMetricsResp)
	}
	return dbMetrics, nil
}

// fetchNetworkMetrics fetches network metrics from the Prometheus HTTP API.
func (app *App) fetchNetworkMetrics() (map[string]models.NetworkPromResp, error) {
	networkMetrics := make(map[string]models.NetworkPromResp)

	for _, host := range app.networkSvc.hosts {
		networkMetricsResp := models.NetworkPromResp{}
		for metric, query := range app.networkSvc.queries {
			switch metric {
			case "packet_errors":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host, host))
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				networkMetricsResp.PacketErrors = value

			default:
				app.lo.Warn("Unknown network metric queried",
					"host", host,
					"metric", metric)
			}
		}

		// Add host metrics to the map.
		networkMetrics[host] = networkMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "data", networkMetricsResp)
	}

	return networkMetrics, nil
}

// pushHWMetrics pushes hardware metrics to the NSE.
func (app *App) pushHWMetrics(host string, data models.HWPromResp) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := app.nseMgr.PushHWMetrics(host, data); err != nil {
			// Handle retry logic.
			if i < app.opts.MaxRetries-1 {
				app.lo.Error("Failed to push hardware metrics to NSE. Retrying...",
					"host", host,
					"attempt", i+1,
					"error", err)
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push hardware metrics to NSE after max retries",
				"host", host,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		break
	}
	return nil
}

// pushDBMetrics pushes hardware metrics to the NSE.
func (app *App) pushDBMetrics(host string, data models.DBPromResp) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := app.nseMgr.PushDBMetrics(host, data); err != nil {
			// Handle retry logic.
			if i < app.opts.MaxRetries-1 {
				app.lo.Error("Failed to push database metrics to NSE. Retrying...",
					"host", host,
					"attempt", i+1,
					"error", err)
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push database metrics to NSE after max retries",
				"host", host,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		break
	}
	return nil
}

// pushNetworkMetrics pushes network metrics to the NSE.
func (app *App) pushNetworkMetrics(host string, data models.NetworkPromResp) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := app.nseMgr.PushNetworkMetrics(host, data); err != nil {
			// Handle retry logic.
			if i < app.opts.MaxRetries-1 {
				app.lo.Error("Failed to push network metrics to NSE. Retrying...",
					"host", host,
					"attempt", i+1,
					"error", err)
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push network metrics to NSE after max retries",
				"host", host,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		break
	}
	return nil
}
