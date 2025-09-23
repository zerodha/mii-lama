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
	benchmark float64
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

func (app *App) fetchCapacityMetrics() (map[int]map[string]models.CapacityPromResp, error) {
	capacityMetrics := make(map[int]map[string]models.CapacityPromResp)

	for locationID := range app.capacitySvc.hosts {
		capacityMetrics[locationID] = make(map[string]models.CapacityPromResp)

		for metric, query := range app.capacitySvc.queries {
			switch metric {
			case "orders_count":
				values, err := app.metricsMgr.QueryMap(query, "segment")
				if err != nil {
					app.lo.Error("Failed to fetch capacity metrics from Prometheus", "error", err)
					continue
				}

				for segment, value := range values {
					if _, exists := capacityMetrics[locationID][segment]; !exists {
						capacityMetrics[locationID][segment] = models.CapacityPromResp{Segment: segment}
					}
					resp := capacityMetrics[locationID][segment]
					resp.OrdersCount = value
					capacityMetrics[locationID][segment] = resp
				}

			default:
				app.lo.Warn("Unknown capacity metric", "metric", metric)
			}
		}

		// Calculate utilization for each segment
		for segment, resp := range capacityMetrics[locationID] {
			if app.capacitySvc.benchmark > 0 {
				resp.Utilization = (resp.OrdersCount / app.capacitySvc.benchmark) * 100
				capacityMetrics[locationID][segment] = resp
			}
		}
	}

	return capacityMetrics, nil
}

func (app *App) retryPushWithSequenceSync(operation func() error, operationType, host string, locationID int) error {
	for i := 0; i < app.opts.MaxRetries; i++ {
		if err := operation(); err != nil {
			if i < app.opts.MaxRetries-1 {
				if strings.Contains(err.Error(), "sequence ID") && strings.Contains(err.Error(), "updated") {
					app.lo.Debug("Retrying after sequence sync", "type", operationType)
				} else {
					app.lo.Warn("Push failed, retrying", "type", operationType, "error", err)
				}
				time.Sleep(app.opts.RetryInterval)
				continue
			}
			app.lo.Error("Push failed after retries", "type", operationType, "error", err)
			return err
		}
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

// getSegmentID maps segment names to NSE segment IDs based on patterns:
// 1=Capital Markets (NSE, BSE), 2=F&O (all -FUT/-OPT), 3=Currency Derivatives (CDS), 4=Commodity (MCX)
func getSegmentID(segmentName string) int {
	switch {
	case segmentName == "NSE" || segmentName == "BSE":
		return 1 // Capital Markets
	case strings.Contains(segmentName, "CDS"):
		return 3 // Currency Derivatives
	case strings.Contains(segmentName, "MCX"):
		return 4 // Commodity
	case strings.Contains(segmentName, "-FUT") || strings.Contains(segmentName, "-OPT"):
		return 2 // F&O
	default:
		return 1 // Default to Capital Markets
	}
}

func (app *App) pushCapacityMetrics(locationID int, host string, data models.CapacityPromResp) error {
	segmentID := getSegmentID(data.Segment)

	return app.retryPushWithSequenceSync(
		func() error {
			return app.nseMgr.PushCapacityMetrics(locationID, host, data, app.capacitySvc.benchmark, segmentID)
		},
		"capacity", host, locationID)
}
