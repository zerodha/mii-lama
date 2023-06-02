<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" /></a>

# mii-lama

`mii-lama` is a Golang based utility, developed to bridge the gap between [Prometheus](https://prometheus.io/)-based metrics and the LAMA API Gateway.

## Features

- Extract metrics from your existing Prometheus-compatible storage solution with the help of pre-defined [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) queries.
- Convert these extracted metrics into the LAMA spec format, ensuring compatibility with the LAMA API Gateway.
- Periodically fetch and send these transformed metrics to the LAMA API Gateway via HTTP.

## Installation

You can grab the latest binaries for Linux, MacOS and Windows from the [Releases](https://github.com/zerodha/mii-lama/releases) section.

## Setting Up

### node-exporter

`mii-lama` uses metrics exposed by `node-exporter`. The following steps will help you set up `mii-lama` with `node-exporter` on all the instances that require to be monitored. Follow these steps for installing and running `node-exporter` on each host:

#### Install node-exporter

1. **Download Node Exporter:** Node Exporter can be downloaded from the Prometheus official site. Visit the [Prometheus download page](https://prometheus.io/download/#node_exporter) and download the latest version of Node Exporter compatible with your system.

2. **Extract the downloaded file:** Use tar to extract the downloaded file. For example, if you downloaded the file to your Downloads directory, you can use the following command:

    ```
    tar -xvf ~/Downloads/node_exporter-*.*-amd64.tar.gz
    ```

3. **Move the Node Exporter binary:** Move the `node_exporter` binary to a directory on your system path, like `/usr/local/bin`. For example:

    ```
    mv node_exporter-*.*-amd64/node_exporter /usr/local/bin/
    ```

4. **Clean up:** After you've moved the binary, you can delete the rest of the extracted contents:

    ```
    rm -r node_exporter-*.*-amd64
    ```

#### Running Node Exporter as a service

For production use, it's recommended to run Node Exporter as a system service. If you're using a system with systemd, you can follow these steps to run Node Exporter as a service:

1. **Create a systemd service file for Node Exporter:** Open a new service file for Node Exporter with a command like:

    ```
    sudo nano /etc/systemd/system/node_exporter.service
    ```

    Then, add the following contents to the file:

    ```
    [Unit]
    Description=Node Exporter
    Requires=node_exporter.socket

    [Service]
    User=node_exporter
    EnvironmentFile=/etc/sysconfig/node_exporter
    ExecStart=/usr/sbin/node_exporter --web.systemd-socket $OPTIONS

    [Install]
    WantedBy=multi-user.target
    ```

    Save and close the file when you're done.

2. **Reload systemd:** After you've created the service file, tell systemd to reload its configuration with:

    ```
    sudo systemctl daemon-reload
    ```

3. **Start Node Exporter:** Start the Node Exporter service with:

    ```
    sudo systemctl start node_exporter
    ```

4. **Enable Node Exporter on boot:** If you want Node Exporter to start automatically when your system boots, use:

    ```
    sudo systemctl enable node_exporter
    ```

You can check the status of the Node Exporter service at any time with the following command:

```
sudo systemctl status node_exporter
```

This should show that the service is active (running). This starts Node Exporter on its default port, 9100. You can check if Node Exporter is running by visiting `http://localhost:9100/metrics` in your web browser. This page displays the raw metrics that Node Exporter exposes to Prometheus.


### Storing Metrics

The metrics exposed by `node-metrics` on all host machines should be centrally shipped and stored on a central metrics timeseries database. `node-exporter` exports metrics in Prometheus format so any Prometheus API compatible storage system will work.

The [docker-compose.yml](./deploy/docker-compose.yml) shows an example of how to spin up a Prometheus server to store metrics.


#### Configuring Prometheus

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

#### Configuring mii-lama

`mii-lama` needs a config file to specify which hosts' metrics need to be collected. This can be specified in `metrics.hardware.hosts` section:

```tom
hosts = ["kite.db.internal:9100"]
```

### Running the stack

Once the above configuration is in-place, the following command will spin up Prometheus server and `mii-lama` agent as docker containers. 

```
cd deploy
docker-compose up
```

Monitor the logs of both the services and ensure that the metrics are being collected. To push the metrics to LAMA API gateway, configure the following section with proper credentials:

```toml
[lama.nse]
url = "https://lama.nse.internal" # Endpoint for NSE LAMA API Gateway
login_id = "redacted"
member_id = "redacted"
password = "redacted"
```

## Advanced Instructions

Please refer to [advanced instructions](./docs/usage.md) for advanced usage instructions.

## Contributing

Please feel free to open issues for feature requests, bug fixes or general suggestions. Pull requests are welcome too.
## License

[LICENSE](./LICENSE)
