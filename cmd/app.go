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

	hardwareSvc    *hardwareService
	dbSvc          *dbService
	networkSvc     *networkService
	applicationSvc *applicationService
}

type Opts struct {
	MaxRetries    int
	RetryInterval time.Duration
	SyncInterval  time.Duration
}

type HostConfig map[int]string

type hardwareService struct {
	hosts   HostConfig
	queries map[string]string
}

type dbService struct {
	hosts   HostConfig
	queries map[string]string
}

type networkService struct {
	hosts   HostConfig
	queries map[string]string
}

type applicationService struct {
	hosts   HostConfig
	queries map[string]string
}

func (app *App) fetchHWMetrics() (map[int]models.HWPromResp, error) {
	hwMetrics := make(map[int]models.HWPromResp)

	for locationID, host := range app.hardwareSvc.hosts {
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

		hwMetrics[locationID] = hwMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "locationID", locationID, "data", hwMetricsResp)
	}

	return hwMetrics, nil
}

func (app *App) fetchDBMetrics() (map[int]models.DBPromResp, error) {
	dbMetrics := make(map[int]models.DBPromResp)

	for locationID, host := range app.dbSvc.hosts {
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

		dbMetrics[locationID] = dbMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "locationID", locationID, "data", dbMetricsResp)
	}
	return dbMetrics, nil
}

func (app *App) fetchNetworkMetrics() (map[int]models.NetworkPromResp, error) {
	networkMetrics := make(map[int]models.NetworkPromResp)

	for locationID, host := range app.networkSvc.hosts {
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

		networkMetrics[locationID] = networkMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "locationID", locationID, "data", networkMetricsResp)
	}

	return networkMetrics, nil
}

func (app *App) fetchApplicationMetrics() (map[int]models.AppPromResp, error) {
	appMetrics := make(map[int]models.AppPromResp)

	for locationID, host := range app.applicationSvc.hosts {
		appMetricsResp := models.AppPromResp{}
		for metric, query := range app.applicationSvc.queries {
			switch metric {
			case "throughput":
				value, err := app.metricsMgr.Query(query)
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				appMetricsResp.Throughput = value

			case "failure_count":
				value, err := app.metricsMgr.Query(query)
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				appMetricsResp.FailureCount = value

			default:
				app.lo.Warn("Unknown application metric queried",
					"host", host,
					"metric", metric)
			}
		}

		appMetrics[locationID] = appMetricsResp
		app.lo.Debug("fetched metrics", "host", host, "locationID", locationID, "data", appMetricsResp)
	}

	return appMetrics, nil
}

func (app *App) pushHWMetrics(locationID int, host string, data models.HWPromResp) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := app.nseMgr.PushHWMetrics(locationID, host, data); err != nil {
			if i < app.opts.MaxRetries-1 {
				app.lo.Error("Failed to push hardware metrics to NSE. Retrying...",
					"host", host,
					"locationID", locationID,
					"attempt", i+1,
					"error", err)
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push hardware metrics to NSE after max retries",
				"host", host,
				"locationID", locationID,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		break
	}
	return nil
}

func (app *App) pushDBMetrics(locationID int, host string, data models.DBPromResp) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := app.nseMgr.PushDBMetrics(locationID, host, data); err != nil {
			if i < app.opts.MaxRetries-1 {
				app.lo.Error("Failed to push database metrics to NSE. Retrying...",
					"host", host,
					"locationID", locationID,
					"attempt", i+1,
					"error", err)
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push database metrics to NSE after max retries",
				"host", host,
				"locationID", locationID,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		break
	}
	return nil
}

func (app *App) pushNetworkMetrics(locationID int, host string, data models.NetworkPromResp) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := app.nseMgr.PushNetworkMetrics(locationID, host, data); err != nil {
			if i < app.opts.MaxRetries-1 {
				app.lo.Error("Failed to push network metrics to NSE. Retrying...",
					"host", host,
					"locationID", locationID,
					"attempt", i+1,
					"error", err)
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push network metrics to NSE after max retries",
				"host", host,
				"locationID", locationID,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		break
	}
	return nil
}

func (app *App) pushApplicationMetrics(locationID int, host string, data models.AppPromResp) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := app.nseMgr.PushAppMetrics(locationID, host, data); err != nil {
			if i < app.opts.MaxRetries-1 {
				app.lo.Error("Failed to push application metrics to NSE. Retrying...",
					"host", host,
					"locationID", locationID,
					"attempt", i+1,
					"error", err)
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push application metrics to NSE after max retries",
				"host", host,
				"locationID", locationID,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		break
	}
	return nil
}
