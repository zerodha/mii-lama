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
		return fmt.Errorf("could not create request: %v", err)
	}

	// If the username and password are set, add them to the request
	if m.opts.Username != "" && m.opts.Password != "" {
		req.Header.Add("Authorization", "Basic "+generateBasicAuthHeader(m.opts.Username, m.opts.Password))
	}

	// Use the client to send the request
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("error querying tsdb status page: %w", err)
	}
	defer resp.Body.Close()

	// Check the status code of the response: if it's not 200, return an error
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non 200 response from tsdb status page: %d", resp.StatusCode)
	}

	return nil
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
		return 0, fmt.Errorf("could not create request: %v", err)
	}

	req.Header = h

	resp, err := m.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check the status code of the response.
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("http request failed with status code: %d", resp.StatusCode)
	}

	// Unmarshal the JSON response into a PrometheusResponse struct
	var promResp PrometheusResponse
	if err = json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return 0, fmt.Errorf("unmarshalling response failed: %w", err)
	}

	// Check if the response contains any metrics.
	if len(promResp.Data.Result) == 0 {
		return 0, fmt.Errorf("no result found in response")
	}

	// Extract the second entry of the "value" field.
	if len(promResp.Data.Result[0].Value) > 1 {
		value, ok := promResp.Data.Result[0].Value[1].(string)
		if !ok {
			return 0, fmt.Errorf("value is not a string")
		}

		// Convert string to float64.
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("converting string to float failed: %w", err)
		}

		return floatValue, nil
	} else {
		return 0, fmt.Errorf("value field not found in response")
	}
}

// generateBasicAuthHeader generates a basic authentication header given a username and password.
func generateBasicAuthHeader(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
