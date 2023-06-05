# Advanced

## Running Node Exporter as a service

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
