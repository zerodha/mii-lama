package models

// HWPromResp is the response from the Prometheus HTTP API for hardware metrics.
type HWPromResp struct {
	CPU    float64 `json:"cpu"`
	Mem    float64 `json:"mem"`
	Disk   float64 `json:"disk"`
	Uptime float64 `json:"uptime"`
}

// DBPromResp is the response from the Prometheus HTTP API for database metrics.
type DBPromResp struct {
	Status float64 `json:"status"`
}

// NetworkPromResp is the response from the Prometheus HTTP API for network metrics.
type NetworkPromResp struct {
	PacketErrors float64 `json:"packet_errors"`
}

// AppPromResp is the response from the Prometheus HTTP API for application metrics.
type AppPromResp struct {
	Throughput   float64 `json:"throughput"`
	FailureCount float64 `json:"failure_count"`
}

// AppMetric represents an individual application metric.
type AppMetric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}
