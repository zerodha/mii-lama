package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	flag "github.com/spf13/pflag"
	metrics "github.com/zerodha/mii-lama/internal/metrics"
	"github.com/zerodha/mii-lama/internal/nse"
	"golang.org/x/exp/slog"
)

type PromConfig struct {
	ScrapeConfigs []PromScrapeConfig `koanf:"scrape_configs"`
}

type PromScrapeConfig struct {
	JobName       string             `koanf:"job_name"`
	StaticConfigs []PromStaticConfig `koanf:"static_configs"`
}

type PromStaticConfig struct {
	Targets []string `koanf:"targets"`
}

// initConfig loads config to `ko`
// object.
func initConfig(cfgDefault, envPrefix string) (*koanf.Koanf, error) {
	var (
		ko = koanf.New(".")
		f  = flag.NewFlagSet("lama", flag.ContinueOnError)
	)

	// Configure Flags.
	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}

	// Register `--config` flag.
	cfgPath := f.String("config", cfgDefault, "Path to a config file to load.")

	// Parse and Load Flags.
	err := f.Parse(os.Args[1:])
	if err != nil {
		return nil, err
	}

	// Load the config files from the path provided.
	err = ko.Load(file.Provider(*cfgPath), toml.Parser())
	if err != nil {
		return nil, err
	}

	// Load environment variables if the key is given
	// and merge into the loaded config.
	if envPrefix != "" {
		err = ko.Load(env.Provider(envPrefix, ".", func(s string) string {
			return strings.Replace(strings.ToLower(
				strings.TrimPrefix(s, envPrefix)), "__", ".", -1)
		}), nil)
		if err != nil {
			return nil, err
		}
	}

	return ko, nil
}

// initLogger initialies a logger.
func initLogger(lvl string) *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}
	if lvl == "debug" {
		opts.Level = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &opts).WithAttrs([]slog.Attr{slog.String("component", "mii-lama")}))
}

// initMetricsManager initialises the metrics manager.
func initMetricsManager(ko *koanf.Koanf) (*metrics.Manager, error) {
	opts := metrics.Opts{
		Endpoint:        ko.MustString("prometheus.endpoint"),
		QueryPath:       ko.MustString("prometheus.query_path"),
		Username:        ko.String("prometheus.username"),
		Password:        ko.String("prometheus.password"),
		Timeout:         ko.MustDuration("prometheus.timeout"),
		IdleConnTimeout: ko.MustDuration("prometheus.idle_timeout"),
		MaxIdleConns:    ko.MustInt("prometheus.max_idle_conns"),
	}

	metrics := metrics.NewManager(opts)

	if err := metrics.Ping(); err != nil {
		return nil, err
	}

	return metrics, nil
}

// Load hardware metrics queries and hosts from the configuration
func inithardwareSvc(ko *koanf.Koanf) (*hardwareService, error) {
	var (
		queries = map[string]string{
			"cpu":    ko.MustString("metrics.hardware.cpu"),
			"memory": ko.MustString("metrics.hardware.memory"),
			"disk":   ko.MustString("metrics.hardware.disk"),
			"uptime": ko.MustString("metrics.hardware.uptime"),
		}
		hosts   = ko.Strings("metrics.hardware.hosts")
		cfgPath = ko.String("prometheus.config_path")
	)

	// If no hosts are provided, try to load from the prometheus config.
	if len(hosts) == 0 && cfgPath != "" {
		// Fallback to the default hosts from the config.
		// Load the config files from the path provided.
		defaultHosts, err := initDefaultHosts(ko, cfgPath)
		if err != nil {
			return nil, err
		}
		hosts = defaultHosts
	}

	// Validate that hosts are loaded.
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts found in the config")
	}

	return &hardwareService{
		hosts:   hosts,
		queries: queries,
	}, nil
}

// Load hardware metrics queries and hosts from the configuration
func initDBSvc(ko *koanf.Koanf) (*dbService, error) {
	var (
		queries = map[string]string{
			"status": ko.MustString("metrics.database.status"),
		}
		hosts   = ko.Strings("metrics.database.hosts")
		cfgPath = ko.String("prometheus.config_path")
	)

	// If no hosts are provided, try to load from the prometheus config.
	if len(hosts) == 0 && cfgPath != "" {
		// Fallback to the default hosts from the config.
		// Load the config files from the path provided.
		defaultHosts, err := initDefaultHosts(ko, cfgPath)
		if err != nil {
			return nil, err
		}
		hosts = defaultHosts
	}

	// Validate that hosts are loaded.
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts found in the config")
	}

	return &dbService{
		hosts:   hosts,
		queries: queries,
	}, nil
}

// initNSEManager initialises the NSE manager.
func initNSEManager(ko *koanf.Koanf, lo *slog.Logger) (*nse.Manager, error) {
	nseMgr, err := nse.New(lo, nse.Opts{
		URL:        ko.MustString("lama.nse.url"),
		LoginID:    ko.MustString("lama.nse.login_id"),
		MemberID:   ko.MustString("lama.nse.member_id"),
		ExchangeID: ko.MustInt("lama.nse.exchange_id"),
		Password:   ko.MustString("lama.nse.password"),
		Timeout:    ko.MustDuration("lama.nse.timeout"),
	})
	if err != nil {
		return nil, err
	}

	// Attempt a login to NSE API.
	if err := nseMgr.Login(); err != nil {
		return nil, fmt.Errorf("failed to login to NSE API: %v", err)
	}

	return nseMgr, nil
}

func initOpts(ko *koanf.Koanf) Opts {
	return Opts{
		MaxRetries:    ko.MustInt("app.max_retries"),
		RetryInterval: ko.MustDuration("app.retry_interval"),
		SyncInterval:  ko.MustDuration("app.sync_interval"),
	}
}

func initDefaultHosts(ko *koanf.Koanf, cfgPath string) ([]string, error) {
	var (
		promCfg      PromConfig
		defaultHosts []string
	)

	// Load the Prometheus config from the path provided for parsing list of default hosts.
	if err := ko.Load(file.Provider(cfgPath), yaml.Parser()); err != nil {
		return nil, err
	}

	if err := ko.Unmarshal("", &promCfg); err != nil {
		return nil, err
	}

	for _, scrapeConfig := range promCfg.ScrapeConfigs {
		// Find the scrape config for the metrics-db job.
		if scrapeConfig.JobName == "metrics-db" {
			for _, staticConfig := range scrapeConfig.StaticConfigs {
				defaultHosts = append(defaultHosts, staticConfig.Targets...)
			}
		}
	}

	return defaultHosts, nil
}
