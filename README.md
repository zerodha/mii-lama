<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" /></a>

# mii-lama

`mii-lama` is a tool that aggregates system metrics (eg: CPU, RAM etc.) from any number of Linux or Windows servers, aggregates them, and posts them to the LAMA (Logging and Monitoring Mechanism) framework operated by Indian stock exchanges.

It involves three components.

- **node_exporter**: A lightweight program (part of the popular Prometheus project) that is to installed on all servers from where metrics are to be collected. It exposes the metrics on port `9100`.

- **prometheus**: A timeseries database (DB) that collects and stores the metrics from all node_exporter agents installed on servers. It allows collected metrics and queries to be transformed in any manner.

- **mii-lama**: The program collects metrics from prometheus DB, converts it to the LAMA API spec, and uploads it to the exchange systems at regular intervals.

![image](https://github.com/zerodha/mii-lama/assets/547147/7b7c13a6-e7c9-4197-80a7-705639760974)



# Installation

## 1. Download programs

- Download the latest release of `node_exporter` ([Linux](https://prometheus.io/download/#node_exporter), [Windows](https://github.com/prometheus-community/windows_exporter/releases)). Extract the binary, copy it to all servers that need to be monitored and leave the binary running on them (Don't forget to open TCP port 9100 on the internal network on the servers).

- Download and install [Docker](https://www.docker.com/get-started/) on the centralised system (master) where `mii-lama` will run.

## 2. Download config files
- Download [docker-compose.yml](https://github.com/zerodha/mii-lama/blob/main/demo/docker-compose.yml?raw=true) from this repository and save it on the master system. This will run prometheus DB and mii-lama out-of-the box inside a Docker container.

- Download [prometheus.yml](https://raw.githubusercontent.com/zerodha/mii-lama/main/deploy/prometheus/prometheus.yml) configuration file and save it alongside `docker-compose.yml` on the master system.

- Download [config.sample.toml](https://raw.githubusercontent.com/zerodha/mii-lama/main/config.sample.toml) and save it alongside `docker-compose.yml` as `config.toml` on the master system.


## 2. Configure

- Edit `prometheus.yml` and add the IPs of all the servers on which node_exporter is installed and have to be monitored. Eg: `"192.168.0.1:9100", "192.168.0.2:9100"`

- Edit `config.toml`. Add the exchange API credentials to the `[lama.*]` section and change the rest of the config optionally. See [advanced configuration](./docs/config.md) for more info.

## 3. Run
- Ensure that the `node_exporter` service is running on all the servers to be monitored as a background service (on Linux and Windows). For Windows, see instructions [here](https://github.com/prometheus-community/windows_exporter).

- On the master system, run `docker-compose up -d` to start the Prometheus DB and mii-lama in the background. Prometheus will start collecting metrics from the node_exporter endpoints configured in `prometheus.yml` and `mii-lama` will query and aggregate metrics from it based on the queries defined in `config.toml` and start posting periodically to the exchange systems.

To ensure that everything is working correctly, inspect the stdout/logs from the programs with the following commands:
- `docker-compose logs -f` for all logs.
- `docker-compose logs -f prometheus` for Prometheus logs.
- `docker-compose logs -f mii-lama` for mii-lama logs.


# Advanced

Please refer to [advanced instructions](./docs/advanced.md) for advanced usage instructions.

## Contributing

Please feel free to open issues for feature requests, bug fixes or general suggestions. Pull requests are welcome too.
## License

[LICENSE](./LICENSE)
