# Configuration

You can configure `mii-lama` by creating a `config.toml` file with the following fields. Please refer to the [example config](./config.sample.toml) for more details.

| Config Field                | Description                                                                                                                                           | Example                             |
| --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| `app.log_level`             | Defines the level of logging. For debug logging, set this value to `debug`.                                                                           | `debug`                             |
| `app.sync_interval`         | Sets the interval at which the application fetches data from the metrics store. The value must be in a format that time.ParseDuration can understand. | `5m`                                |
| `app.retry_interval`        | Defines the interval at which the application retries a failed request. The value must be in a format that time.ParseDuration can understand.         | `5s`                                |
| `app.max_retries`           | Defines the maximum number of retries for a failed request.                                                                                           | `3`                                 |
| `lama.nse.url`              | Sets the URL for the LAMA NSE API Gateway.                                                                                                            | `https://lama.nse.internal`         |
| `lama.nse.login_id`         | Defines the login ID for the LAMA NSE API Gateway.                                                                                                    | `redacted`                          |
| `lama.nse.member_id`        | Sets the member ID for the LAMA NSE API Gateway.                                                                                                      | `redacted`                          |
| `lama.nse.password`         | Defines the password for the LAMA NSE API Gateway.                                                                                                    | `redacted`                          |
| `lama.nse.timeout`          | Sets the timeout for HTTP requests to the LAMA NSE API Gateway. The value must be in a format that time.ParseDuration can understand.                 | `30s`                               |
| `lama.nse.exchange_id`      | Defines the exchange ID for the LAMA NSE API Gateway.                                                                                                 | `1`                                 |
| `prometheus.endpoint`       | Sets the URL for the Prometheus API.                                                                                                                  | `http://prometheus.broker.internal` |
| `prometheus.query_path`     | Defines the endpoint for the Prometheus query API.                                                                                                    | `/api/v1/query`                     |
| `prometheus.username`       | Sets the username for HTTP Basic Auth when accessing the Prometheus API.                                                                              | `redacted`                          |
| `prometheus.password`       | Defines the password for HTTP Basic Auth when accessing the Prometheus API.                                                                           | `redacted`                          |
| `prometheus.timeout`        | Sets the timeout for HTTP requests to the Prometheus API. The value must be in a format that time.ParseDuration can understand.                       | `10s`                               |
| `prometheus.max_idle_conns` | Defines the maximum number of idle connections to the Prometheus API.                                                                                 | `10`                                |
| `metrics.hardware.hosts`    | A list of hosts from which to gather metrics.                                                                                                         | `["kite-db-172.x.y.z"]`             |
| `metrics.hardware.cpu`      | Defines the Prometheus query for gathering CPU usage metrics.                                                                                         | Refer to config                     |
| `metrics.hardware.memory`   | Sets the Prometheus query for gathering memory usage metrics.                                                                                         | Refer to config                     |
| `metrics.hardware.disk`     | Defines the Prometheus query for gathering disk usage metrics.                                                                                        | Refer to config                     |
| `metrics.hardware.uptime`   | Sets the Prometheus query for gathering system uptime metrics.                                                                                        | Refer to config                     |


Please replace all instances of `"redacted"` with your actual credentials or values. Also, remember to replace `"%s"` placeholders in the Prometheus queries with your actual hostnames.


## Configuring Prometheus

The default config file for Prometheus is located at [prometheus.yml](./deploy/prometheus/prometheus.yml). For each host machine, you need to add a section in `scrape_configs`. Here's an example:

```yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "kite-db"
    scrape_interval: 5s
    static_configs:
      - targets: ["kite.db.internal:9100"]
```

This config will _pull_ the metrics from the host `kite.db.internal` on port `9100` where `node-exporter` is running.


## Configuring endpoints for LAMA API gateway


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

## Configuring endpoints for Prometheus

To use `mii-lama`, you need to have a working instance of a Prometheus-compatible storage system (like Thanos or VictoriaMetrics) and access to a LAMA API Gateway credentials.

All Prometheus-compatible storage systems expose metrics via a [HTTP endpoint](https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries) (`/api/v1/query`). `mii-lama` uses this endpoint to fetch metrics from the storage system. The endpoint URL and credentials (if any) are specified in the `config.toml` file.

```toml
[prometheus]
endpoint = "http://prometheus.broker.internal" # Endpoint for Prometheus API
username = "redacted" # Optional Basic Auth credentials
password = "redacted" # Optional Basic Auth credentials
```

`mii-lama` supports not just Prometheus, but any storage system that is compatible with Prometheus [remote_write](https://prometheus.io/docs/practices/remote_write/) API specification. Some examples of such systems are [Grafana Mimir](https://grafana.com/oss/mimir/) and [VictoriaMetrics](https://victoriametrics.com/).