package metrics

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Opts struct {
	Endpoint        string
	QueryPath       string
	Username        string
	Password        string
	IdleConnTimeout time.Duration
	Timeout         time.Duration
	MaxIdleConns    int
	DefaultHosts    []string
}

type Manager struct {
	client *http.Client
	opts   Opts
}

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
	Stats struct {
		SeriesFetched string `json:"seriesFetched"`
	} `json:"stats"`
}

// NewManager returns a new metrics manager.
func NewManager(opts Opts) *Manager {
	client := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:    opts.MaxIdleConns,
			IdleConnTimeout: opts.IdleConnTimeout,
		},
	}

	return &Manager{
		client: client,
		opts:   opts,
	}
}

// Ping queries the Prometheus HTTP API and checks if the server is up.
func (m *Manager) Ping() error {
	var (
		endpoint = m.opts.Endpoint + "/api/v1/status/tsdb"
	)

	// Create a new request using http
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %v", err)
	}

	// If the username and password are set, add them to the request
	if m.opts.Username != "" && m.opts.Password != "" {
		req.Header.Add("Authorization", "Basic "+generateBasicAuthHeader(m.opts.Username, m.opts.Password))
	}

	// Use the client to send the request
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to query tsdb status page: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code of the response: if it's not 200, return an error
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response (%d) from tsdb status page", resp.StatusCode)
	}

	return nil
}

// QueryMap queries the Prometheus HTTP API and returns a map of label values to metric values.
// For queries that return multiple results (e.g., grouped by a label), this returns a map.
func (m *Manager) QueryMap(query string, labelKey string) (map[string]float64, error) {
	var (
		root_url = m.opts.Endpoint + m.opts.QueryPath
		h        = http.Header{}
		params   = url.Values{}
		result   = make(map[string]float64)
	)

	params.Add("query", query)
	params.Add("time", strconv.FormatInt(time.Now().Unix(), 10))

	// Set the username and password for basic authentication.
	if m.opts.Username != "" && m.opts.Password != "" {
		auth := generateBasicAuthHeader(m.opts.Username, m.opts.Password)
		h.Set("Authorization", "Basic "+auth)
	}

	reqUrl := root_url + "?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %v", err)
	}

	req.Header = h

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code of the response.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request returned a non-200 status code (%d)", resp.StatusCode)
	}

	// Unmarshal the JSON response into a PrometheusResponse struct
	var promResp PrometheusResponse
	if err = json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the response body: %v", err)
	}

	// Check if the response contains any metrics.
	if len(promResp.Data.Result) == 0 {
		return nil, fmt.Errorf("response contains no result data")
	}

	// Extract values for each result
	for _, res := range promResp.Data.Result {
		labelValue, ok := res.Metric[labelKey]
		if !ok {
			return nil, fmt.Errorf("label %s not found in metric", labelKey)
		}

		if len(res.Value) > 1 {
			value, ok := res.Value[1].(string)
			if !ok {
				return nil, fmt.Errorf("value in the response is not a string")
			}

			// Convert string to float64.
			floatValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to convert response value from string to float64: %v", err)
			}

			result[labelValue] = floatValue
		} else {
			return nil, fmt.Errorf("response contains no 'value' field")
		}
	}

	return result, nil
}

// Query queries the Prometheus HTTP API and returns the metric value.
func (m *Manager) Query(query string) (float64, error) {
	var (
		root_url = m.opts.Endpoint + m.opts.QueryPath
		h        = http.Header{}
		params   = url.Values{}
	)

	params.Add("query", query)
	params.Add("time", strconv.FormatInt(time.Now().Unix(), 10))

	// Set the username and password for basic authentication.
	if m.opts.Username != "" && m.opts.Password != "" {
		auth := generateBasicAuthHeader(m.opts.Username, m.opts.Password)
		h.Set("Authorization", "Basic "+auth)
	}

	reqUrl := root_url + "?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create new HTTP request: %v", err)
	}

	req.Header = h

	resp, err := m.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code of the response.
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP request returned a non-200 status code (%d)", resp.StatusCode)
	}

	// Unmarshal the JSON response into a PrometheusResponse struct
	var promResp PrometheusResponse
	if err = json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return 0, fmt.Errorf("failed to unmarshal the response body: %v", err)
	}

	// Check if the response contains any metrics.
	if len(promResp.Data.Result) == 0 {
		return 0, fmt.Errorf("response contains no result data")
	}

	// Extract the second entry of the "value" field.
	if len(promResp.Data.Result[0].Value) > 1 {
		value, ok := promResp.Data.Result[0].Value[1].(string)
		if !ok {
			return 0, fmt.Errorf("value in the response is not a string")
		}

		// Convert string to float64.
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to convert response value from string to float64: %v", err)
		}

		return floatValue, nil
	} else {
		return 0, fmt.Errorf("response contains no 'value' field")
	}
}

// generateBasicAuthHeader generates a basic authentication header given a username and password.
func generateBasicAuthHeader(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
