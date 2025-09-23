package main

import (
	"context"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/zerodha/mii-lama/pkg/models"
)

var (
	// Version of the build. This is injected at build-time.
	buildString = "unknown"
	exit        = func() { os.Exit(1) }
)

func main() {
	// Initialise and load the config.
	ko, err := initConfig("config.sample.toml", "MII_LAMA_")
	if err != nil {
		panic(err.Error())
	}

	lo := initLogger(ko.MustString("app.log_level"))
	lo.Info("booting mii-lama version", "version", buildString)

	// Initialise the metrics manager.
	metricsMgr, err := initMetricsManager(ko)
	if err != nil {
		lo.Error("failed to init metrics manager", "error", err)
		exit()
	}

	// Load queries for hardware metrics.
	hardwareSvc, err := inithardwareSvc(ko)
	if err != nil {
		lo.Error("failed to init hardware service", "error", err)
		exit()
	}
	// Load queries for database metrics.
	dbSvc, err := initDBSvc(ko)
	if err != nil {
		lo.Error("failed to init db service", "error", err)
		exit()
	}

	// Load queries for network metrics.
	networkSvc, err := initNetworkSvc(ko)
	if err != nil {
		lo.Error("failed to init network service", "error", err)
		exit()
	}

	// Load queries for application metrics.
	applicationSvc, err := initApplicationSvc(ko)
	if err != nil {
		lo.Error("failed to init application service", "error", err)
		exit()
	}

	// Load queries for capacity metrics.
	capacitySvc, err := initCapacitySvc(ko)
	if err != nil {
		lo.Error("failed to init capacity service", "error", err)
		exit()
	}

	// Initialise the NSE manager.
	nseMgr, err := initNSEManager(ko, lo)
	if err != nil {
		lo.Error("failed to init nse manager", "error", err)
		exit()
	}

	// Init the app.
	app := &App{
		lo:             lo,
		opts:           initOpts(ko),
		metricsMgr:     metricsMgr,
		nseMgr:         nseMgr,
		hardwareSvc:    hardwareSvc,
		dbSvc:          dbSvc,
		networkSvc:     networkSvc,
		applicationSvc: applicationSvc,
		capacitySvc:    capacitySvc,
	}

	// Create a new context which is cancelled when `SIGINT`/`SIGTERM` is received.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Start the workers for fetching different metrics in the background.
	var wg = &sync.WaitGroup{}

	wg.Add(1)
	go app.syncHWMetricsWorker(ctx, wg)

	wg.Add(1)
	go app.syncDBMetricsWorker(ctx, wg)

	wg.Add(1)
	go app.syncNetworkMetricsWorker(ctx, wg)

	wg.Add(1)
	go app.syncApplicationMetricsWorker(ctx, wg)

	wg.Add(1)
	go app.syncCapacityMetricsWorker(ctx, wg)

	// Listen on the close channel indefinitely until a
	// `SIGINT` or `SIGTERM` is received.
	<-ctx.Done()
	// Cancel the context to gracefully shutdown and perform
	// any cleanup tasks.
	cancel()
	// Wait for all workers to finish.
	wg.Wait()

	app.lo.Info("shutting down")
}

func (app *App) syncHWMetricsWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(app.opts.SyncInterval)
	defer ticker.Stop()

	app.lo.Info("Starting hardware metrics worker", "interval", app.opts.SyncInterval)
	for {
		select {
		case <-ticker.C:
			data, err := app.fetchHWMetrics()
			if err != nil {
				app.lo.Error("Failed to fetch HW metrics", "error", err)
				continue
			}

			// Push to upstream LAMA APIs.
			for locationID, hostData := range data {
				if err := app.pushHWMetrics(locationID, app.hardwareSvc.hosts[locationID], hostData); err != nil {
					app.lo.Error("Failed to push HW metrics to NSE", "locationID", locationID, "error", err)
					continue
				}
			}
		case <-ctx.Done():
			app.lo.Info("Stopping HW metrics worker")
			return
		}
	}
}

func (app *App) syncDBMetricsWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(app.opts.SyncInterval)
	defer ticker.Stop()

	app.lo.Info("Starting DB metrics worker", "interval", app.opts.SyncInterval)
	for {
		select {
		case <-ticker.C:
			data, err := app.fetchDBMetrics()
			if err != nil {
				app.lo.Error("Failed to fetch DB metrics", "error", err)
				continue
			}

			// Push to upstream LAMA APIs.
			for locationID, hostData := range data {
				if err := app.pushDBMetrics(locationID, app.dbSvc.hosts[locationID], hostData); err != nil {
					app.lo.Error("Failed to push DB metrics to NSE", "locationID", locationID, "error", err)
					continue
				}
			}
		case <-ctx.Done():
			app.lo.Info("Stopping DB metrics worker")
			return
		}
	}
}

func (app *App) syncNetworkMetricsWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(app.opts.SyncInterval)
	defer ticker.Stop()

	app.lo.Info("Starting network metrics worker", "interval", app.opts.SyncInterval)
	for {
		select {
		case <-ticker.C:
			data, err := app.fetchNetworkMetrics()
			if err != nil {
				app.lo.Error("Failed to fetch network metrics", "error", err)
				continue
			}

			// Push to upstream LAMA APIs.
			for locationID, hostData := range data {
				if err := app.pushNetworkMetrics(locationID, app.networkSvc.hosts[locationID], hostData); err != nil {
					app.lo.Error("Failed to push network metrics to NSE", "locationID", locationID, "error", err)
					continue
				}
			}
		case <-ctx.Done():
			app.lo.Info("Stopping network metrics worker")
			return
		}
	}
}

func (app *App) syncApplicationMetricsWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(app.opts.SyncInterval)
	defer ticker.Stop()

	app.lo.Info("Starting application metrics worker", "interval", app.opts.SyncInterval)
	for {
		select {
		case <-ticker.C:
			data, err := app.fetchApplicationMetrics()
			if err != nil {
				app.lo.Error("Failed to fetch application metrics", "error", err)
				continue
			}

			// Push to upstream LAMA APIs.
			for locationID, hostData := range data {
				if err := app.pushApplicationMetrics(locationID, app.applicationSvc.hosts[locationID], hostData); err != nil {
					app.lo.Error("Failed to push application metrics to NSE", "locationID", locationID, "error", err)
					continue
				}
			}
		case <-ctx.Done():
			app.lo.Info("Stopping application metrics worker")
			return
		}
	}
}

func (app *App) syncCapacityMetricsWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(app.opts.SyncInterval)
	defer ticker.Stop()

	app.lo.Info("Starting capacity metrics worker", "interval", app.opts.SyncInterval)
	for {
		select {
		case <-ticker.C:
			data, err := app.fetchCapacityMetrics()
			if err != nil {
				app.lo.Error("Failed to fetch capacity metrics", "error", err)
				continue
			}

			// Push metrics to NSE for each segment, aggregating by segment ID
			for locationID, segmentData := range data {
				host := app.capacitySvc.hosts[locationID]

				// Aggregate segments by their segment ID
				aggregatedData := make(map[int]models.CapacityPromResp)

				for segment, metricData := range segmentData {
					// Skip segments with no order data or very low counts that round to 0
					roundedOrders := math.Round(metricData.OrdersCount)
					if metricData.OrdersCount <= 0 || roundedOrders <= 0 {
						app.lo.Debug("Skipping segment with low/no orders", "segment", segment, "orders_count", metricData.OrdersCount, "rounded", roundedOrders)
						continue
					}

					segmentID := getSegmentID(segment)
					if existing, exists := aggregatedData[segmentID]; exists {
						// Aggregate orders count for same segment ID
						existing.OrdersCount += metricData.OrdersCount
						aggregatedData[segmentID] = existing
						app.lo.Debug("Aggregating segment data", "segment", segment, "segment_id", segmentID, "additional_orders", metricData.OrdersCount)
					} else {
						// First segment for this ID - create new entry
						aggregatedData[segmentID] = models.CapacityPromResp{
							OrdersCount: metricData.OrdersCount,
							Utilization: metricData.Utilization,
							Segment:     segment, // Store representative segment name
						}
					}
				}

				// Send aggregated data for each segment ID
				for segmentID, metricData := range aggregatedData {
					if err := app.pushCapacityMetrics(locationID, host, metricData); err != nil {
						app.lo.Error("Failed to push capacity metrics to NSE", "segment_id", segmentID, "error", err)
						continue
					}
				}
			}
		case <-ctx.Done():
			app.lo.Info("Stopping capacity metrics worker")
			return
		}
	}
}
