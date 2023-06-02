<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" /></a>

# mii-lama

`mii-lama` is a Golang based utility, developed to bridge the gap between Prometheus-based metrics and the LAMA API Gateway.

## TOC

- [mii-lama](#mii-lama)
  - [TOC](#toc)
  - [Features](#features)
  - [Installation](#installation)
  - [Pre-requisites](#pre-requisites)
  - [Usage](#usage)
  - [Configuration](#configuration)
  - [Considerations](#considerations)
    - [Node Exporter](#node-exporter)
    - [Specifying Hosts](#specifying-hosts)
  - [Contributing](#contributing)
  - [License](#license)


## Features

- Extract metrics from your existing Prometheus-compatible storage solution with the help of pre-defined PromQL queries.
- Convert these extracted metrics into the LAMA spec format, ensuring compatibility with the LAMA API Gateway.
- Periodically fetch and send these transformed metrics to the LAMA API Gateway via HTTP.

## Installation

You can grab the latest binaries for Linux, MacOS and Windows from the [Releases](https://github.com/zerodha/mii-lama/releases) section.

## Pre-requisites

To use `mii-lama`, you need to have a working instance of a Prometheus-compatible storage system (like Thanos or VictoriaMetrics) and access to a LAMA API Gateway credentials.

All Prometheus-compatible storage systems expose metrics via a [HTTP endpoint](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries) (`/api/v1/query`). `mii-lama` uses this endpoint to fetch metrics from the storage system. The endpoint URL and credentials (if any) are specified in the configuration file.

```toml
[prometheus]
endpoint = "http://prometheus.broker.internal" # Endpoint for Prometheus API
username = "redacted" # Optional Basic Auth credentials
password = "redacted" # Optional Basic Auth credentials
```

`mii-lama` supports not just Prometheus, but any storage system that is compatible with Prometheus [remote_write](https://prometheus.io/docs/practices/remote_write/) API specification. Some examples of such systems are [Grafana Mimir](https://grafana.com/oss/mimir/) and [VictoriaMetrics](https://victoriametrics.com/).

`mii-lama` uses the LAMA API Gateway to send metrics to the LAMA platform. The API Gateway URL and credentials (if any) are specified in the configuration file.

```toml
[lama.nse]
url = "https://lama.nse.internal" # Endpoint for NSE LAMA API Gateway
login_id = "redacted"
member_id = "redacted"
password = "redacted"
timeout = "30s" # Timeout for HTTP requests
exchange_id = 1 # 1=National Stock Exchange
```

## Usage

The utility can be run as a standalone binary. The configuration file is passed to the utility via the `--config` flag.

```bash
$ mii-lama --config config.toml
```

## Configuration

You can configure `mii-lama` by creating a `config.toml` file with the following fields. Please refer to the [example config](./config.sample.toml) for more details.

| Section            | Field            | Example                                                                                                                                                                                       | Description                                                                               |
| ------------------ | ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `app`              | `log_level`      | `"debug"`                                                                                                                                                                                     | The logging level. To enable debug logging, set the level as `debug`.                     |
| `app`              | `sync_interval`  | `"5m"`                                                                                                                                                                                        | The interval at which the app should fetch data from the metrics store.                   |
| `app`              | `retry_interval` | `"5s"`                                                                                                                                                                                        | The interval at which the app should retry if the previous request failed.                |
| `app`              | `max_retries`    | `3`                                                                                                                                                                                           | The maximum number of retries for a failed request.                                       |
| `lama.nse`         | `url`            | `"https://lama.nse.internal"`                                                                                                                                                                 | The endpoint for NSE LAMA API Gateway.                                                    |
| `lama.nse`         | `login_id`       | `"redacted"`                                                                                                                                                                                  | The login ID for the LAMA NSE API Gateway.                                                |
| `lama.nse`         | `member_id`      | `"redacted"`                                                                                                                                                                                  | The member ID for the LAMA NSE API Gateway.                                               |
| `lama.nse`         | `password`       | `"redacted"`                                                                                                                                                                                  | The password for the LAMA NSE API Gateway.                                                |
| `lama.nse`         | `timeout`        | `"30s"`                                                                                                                                                                                       | The timeout for HTTP requests to the LAMA NSE API Gateway.                                |
| `lama.nse`         | `exchange_id`    | `1`                                                                                                                                                                                           | The exchange ID for the LAMA NSE API Gateway. `1` corresponds to National Stock Exchange. |
| `prometheus`       | `endpoint`       | `"http://prometheus.broker.internal"`                                                                                                                                                         | The endpoint for Prometheus API.                                                          |
| `prometheus`       | `query_path`     | `"/api/v1/query"`                                                                                                                                                                             | The endpoint for Prometheus query API.                                                    |
| `prometheus`       | `username`       | `"redacted"`                                                                                                                                                                                  | The HTTP Basic Auth username for Prometheus API.                                          |
| `prometheus`       | `password`       | `"redacted"`                                                                                                                                                                                  | The HTTP Basic Auth password for Prometheus API.                                          |
| `prometheus`       | `timeout`        | `"10s"`                                                                                                                                                                                       | The timeout for HTTP requests to the Prometheus API.                                      |
| `prometheus`       | `max_idle_conns` | `10`                                                                                                                                                                                          | The maximum number of idle connections allowed to the Prometheus API.                     |
| `metrics.hardware` | `hosts`          | `["kite-db-172.x.y.z"]`                                                                                                                                                                       | List of hosts for which to fetch hardware metrics.                                        |
| `metrics.hardware` | `cpu`            | `'100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle", hostname="%s"}[5m])))'`                                                                                                             | The PromQL query for fetching CPU usage metrics.                                          |
| `metrics.hardware` | `memory`         | `'(1 - ((node_memory_MemFree_bytes{hostname="%s"} + node_memory_Buffers_bytes{hostname="%s"} + node_memory_Cached_bytes{hostname="%s"}) / node_memory_MemTotal_bytes{hostname="%s"})) * 100'` | The PromQL query for fetching memory usage metrics.                                       |
| `metrics.hardware` | `disk`           | `'100 - ((node_filesystem_avail_bytes{hostname="%s",device!~"rootfs"} * 100) / node_filesystem_size_bytes{hostname="%s",device!~"rootfs"})'`                                                  | The PromQL query for fetching disk usage metrics.                                         |
| `metrics.hardware` | `uptime`         | `'(node_time_seconds{hostname="%s"} - node_boot_time_seconds{hostname="%s"}) / 60'`                                                                                                           | The PromQL query for fetching uptime metrics, converted to minutes.                       |

Please replace all instances of `"redacted"` with your actual credentials or values. Also, remember to replace `"%s"` placeholders in the Prometheus queries with your actual hostnames.



## Considerations

### Node Exporter

`mii-lama` leverages [Node Exporter](https://github.com/prometheus/node_exporter), a widely used Prometheus exporter that provides hardware and OS metrics exposed by *NIX kernels. Node Exporter is written in Go and comes with pluggable metric collectors, making it an excellent choice for this purpose. The config bundles queries for CPU, memory, disk and uptime metrics. You can add more queries to the config as per your requirements.

### Specifying Hosts

Currently the LAMA API spec doesn't have the provision to send a `host` identifier. Due to this, only the first host in the configuration is considered. Once the spec is updated to support, this restriction in `mii-lama` will be removed.

## Contributing

Please feel free to open issues for feature requests, bug fixes or general suggestions. Pull requests are welcome too.
## License

[LICENSE](./LICENSE)
