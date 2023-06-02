package models

// HWPromResp is the response from the Prometheus HTTP API.
type HWPromResp struct {
	CPU    float64 `json:"cpu"`
	Mem    float64 `json:"mem"`
	Disk   float64 `json:"disk"`
	Uptime float64 `json:"uptime"`
}
