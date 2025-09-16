package main

import (
	"fmt"
	"strings"
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
	capacitySvc    *capacityService
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

type capacityService struct {
	hosts     HostConfig
	queries   map[string]string
	benchmark map[string]float64
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

func (app *App) fetchCapacityMetrics() (map[int]models.CapacityPromResp, error) {
	capacityMetrics := make(map[int]models.CapacityPromResp)

	for locationID, host := range app.capacitySvc.hosts {
		capacityMetricsResp := models.CapacityPromResp{}
		for metric, query := range app.capacitySvc.queries {
			switch metric {
			case "orders_count":
				value, err := app.metricsMgr.Query(fmt.Sprintf(query, host))
				if err != nil {
					app.lo.Error("Failed to query Prometheus",
						"host", host,
						"metric", metric,
						"error", err)
					continue
				}
				capacityMetricsResp.OrdersCount = value

			default:
				app.lo.Warn("Unknown capacity metric queried",
					"host", host,
					"metric", metric)
			}
		}

		ordersCapacity := app.capacitySvc.benchmark["orders_per_second"]
		if ordersCapacity > 0 {
			capacityMetricsResp.Utilization = (capacityMetricsResp.OrdersCount / ordersCapacity) * 100
		}

		capacityMetrics[locationID] = capacityMetricsResp
		app.lo.Debug("fetched capacity metrics", "host", host, "locationID", locationID, "data", capacityMetricsResp)
	}

	return capacityMetrics, nil
}

func (app *App) retryPushWithSequenceSync(operation func() error, operationType, host string, locationID int) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := operation(); err != nil {
			if i < app.opts.MaxRetries-1 {
				if strings.Contains(err.Error(), "sequence ID") && strings.Contains(err.Error(), "updated") {
					app.lo.Debug("Sequence ID sync required, retrying",
						"type", operationType,
						"host", host,
						"locationID", locationID,
						"attempt", i+1)
				} else {
					app.lo.Error("Failed to push metrics to NSE. Retrying...",
						"type", operationType,
						"host", host,
						"locationID", locationID,
						"attempt", i+1,
						"error", err)
				}
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Failed to push metrics to NSE after max retries",
				"type", operationType,
				"host", host,
				"locationID", locationID,
				"max_retries", app.opts.MaxRetries,
				"error", err)
			return err
		}
		app.lo.Info("Metrics pushed successfully",
			"type", operationType,
			"host", host,
			"locationID", locationID)
		break
	}
	return nil
}

func (app *App) pushHWMetrics(locationID int, host string, data models.HWPromResp) error {
	return app.retryPushWithSequenceSync(
		func() error { return app.nseMgr.PushHWMetrics(locationID, host, data) },
		"hardware", host, locationID)
}

func (app *App) pushDBMetrics(locationID int, host string, data models.DBPromResp) error {
	return app.retryPushWithSequenceSync(
		func() error { return app.nseMgr.PushDBMetrics(locationID, host, data) },
		"database", host, locationID)
}

func (app *App) pushNetworkMetrics(locationID int, host string, data models.NetworkPromResp) error {
	return app.retryPushWithSequenceSync(
		func() error { return app.nseMgr.PushNetworkMetrics(locationID, host, data) },
		"network", host, locationID)
}

func (app *App) pushApplicationMetrics(locationID int, host string, data models.AppPromResp) error {
	return app.retryPushWithSequenceSync(
		func() error { return app.nseMgr.PushAppMetrics(locationID, host, data) },
		"application", host, locationID)
}

func (app *App) pushCapacityMetrics(locationID int, host string, data models.CapacityPromResp) error {
	ordersCapacity := app.capacitySvc.benchmark["orders_per_second"]
	return app.retryPushWithSequenceSync(
		func() error { return app.nseMgr.PushCapacityMetrics(locationID, host, data, ordersCapacity) },
		"capacity", host, locationID)
}
