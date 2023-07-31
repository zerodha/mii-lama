package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
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
	metricsMgr, err := initMetricsManager(ko,)
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

	// Initialise the NSE manager.
	nseMgr, err := initNSEManager(ko, lo)
	if err != nil {
		lo.Error("failed to init nse manager", "error", err)
		exit()
	}

	// Init the app.
	app := &App{
		lo:          lo,
		opts:        initOpts(ko),
		metricsMgr:  metricsMgr,
		nseMgr:      nseMgr,
		hardwareSvc: hardwareSvc,
		dbSvc:       dbSvc,
		networkSvc:  networkSvc,
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
			for host, hostData := range data {
				if err := app.pushHWMetrics(host, hostData); err != nil {
					app.lo.Error("Failed to push HW metrics to NSE", "host", host, "error", err)
					continue
				}

				// FIXME: Currently the LAMA API does not support multiple hosts.
				// Once we've pushed the data for the first host, break the loop.
				// Once the LAMA API supports multiple hosts, remove this.
				break
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
			for host, hostData := range data {
				if err := app.pushDBMetrics(host, hostData); err != nil {
					app.lo.Error("Failed to push DB metrics to NSE", "host", host, "error", err)
					continue
				}

				// FIXME: Currently the LAMA API does not support multiple hosts.
				// Once we've pushed the data for the first host, break the loop.
				// Once the LAMA API supports multiple hosts, remove this.
				break
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
			for host, hostData := range data {
				if err := app.pushNetworkMetrics(host, hostData); err != nil {
					app.lo.Error("Failed to push network metrics to NSE", "host", host, "error", err)
					continue
				}

				// FIXME: Currently the LAMA API does not support multiple hosts.
				// Once we've pushed the data for the first host, break the loop.
				// Once the LAMA API supports multiple hosts, remove this.
				break
			}
		case <-ctx.Done():
			app.lo.Info("Stopping network metrics worker")
			return
		}
	}
}
