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

	metricsMgr, err := initMetricsManager(ko)
	if err != nil {
		lo.Error("failed to init metrics manager", "error", err)
		exit()
	}

	nseMgr, err := initNSEManager(ko, lo)
	if err != nil {
		lo.Error("failed to init nse manager", "error", err)
		exit()
	}

	hardwareSvc, err := inithardwareSvc(ko)
	if err != nil {
		lo.Error("failed to init hardware service", "error", err)
		exit()
	}

	// Init the app.
	app := &App{
		lo:          lo,
		opts:        initOpts(ko),
		metricsMgr:  metricsMgr,
		nseMgr:      nseMgr,
		hardwareSvc: hardwareSvc,
	}

	// Create a new context which is cancelled when `SIGINT`/`SIGTERM` is received.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Start the worker in background.
	var wg = &sync.WaitGroup{}
	wg.Add(1)
	go app.worker(ctx, wg)

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

func (app *App) worker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(app.opts.SyncInterval)
	defer ticker.Stop()

	app.lo.Info("starting worker", "interval", app.opts.SyncInterval)
	for {
		select {
		case <-ticker.C:
			data, err := app.FetchMetrics()
			if err != nil {
				app.lo.Error("failed to fetch metrics", "error", err)
				continue
			}

			// Push to upstream LAMA APIs.
			for host, hostData := range data {
				// Push to upstream LAMA APIs.
				for i := 0; i < app.opts.MaxRetries; i++ {
					if err := app.nseMgr.PushHWMetrics(host, hostData); err != nil {
						app.lo.Error("failed to push hw metrics to nse", "error", err)
						// Handle retry logic.
						if i < app.opts.MaxRetries-1 {
							time.Sleep(app.opts.RetryInterval)
							app.lo.Info("retrying push to nse", "attempt", i+1)
							continue
						}
						app.lo.Error("failed to push hw metrics to nse", "error", err, "max_retries", app.opts.MaxRetries)
						continue
					}
					break
				}
				// FIXME: Currently the LAMA API does not support multiple hosts.
				// Once we've pushed the data for the first host, break the loop.
				// Once the LAMA API supports multiple hosts, remove this.
				break
			}
		case <-ctx.Done():
			app.lo.Info("quitting worker")
			return
		}
	}
}
