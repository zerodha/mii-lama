package nse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zerodha/mii-lama/pkg/models"
	"golang.org/x/exp/slog"
)

const (
	NSE_RESP_CODE_SUCCESS         = 601
	NSE_RESP_CODE_PARTIAL_SUCCESS = 602
	NSE_RESP_CODE_INVALID_LOGIN   = 701
	NSE_RESP_CODE_INVALID_SEQ_ID  = 704
	NSE_RESP_CODE_INVALID_REQUEST = 706
	NSE_RESP_CODE_INVALID_TOKEN   = 801
	NSE_RESP_CODE_EXPIRED_TOKEN   = 802

	// Default application ID used for metrics
	DEFAULT_APPLICATION_ID = 1
	// Default market segment for capacity metrics
	DEFAULT_MARKET_SEGMENT = 1
)

var (
	// Pre-compiled regex for extracting sequence ID from NSE error messages
	// Handles both formats: "SequenceId should be 123" and "The SequenceId should be 123"
	sequenceIDRegex = regexp.MustCompile(`(?i)(?:the\s+)?sequenceid\s+should\s+be\s+(\d+)`)
)

// sequenceIDSyncHandler handles sequence ID synchronization for different metric types
func (mgr *Manager) sequenceIDSyncHandler(r MetricsResp, metricType string) error {
	expectedSeqID, err := extractExpectedSequenceID(r.ResponseDesc)
	if err != nil {
		// Some NSE endpoints return incomplete error messages without the expected sequence ID
		// In such cases, initialize with 1 (typical API starting sequence ID)
		mgr.lo.Warn("Seq ID extraction failed, using 1", "type", metricType, "error", err)
		expectedSeqID = 1
	} else {
		mgr.lo.Debug("Seq ID sync", "type", metricType, "seq_id", expectedSeqID)
	}

	mgr.Lock()
	defer mgr.Unlock()

	switch metricType {
	case "hardware":
		mgr.hwSeqID = expectedSeqID
	case "database":
		mgr.dbSeqID = expectedSeqID
	case "network":
		mgr.netSeqID = expectedSeqID
	case "application":
		mgr.appSeqID = expectedSeqID
	case "capacity":
		mgr.capSeqID = expectedSeqID
	default:
		return fmt.Errorf("unknown metric type: %s", metricType)
	}

	return fmt.Errorf("seq ID updated to %d, retrying %s", expectedSeqID, metricType)
}

type Opts struct {
	URL             string
	LoginID         string
	MemberID        string
	ExchangeID      int
	Password        string
	Timeout         time.Duration
	IdleConnTimeout time.Duration
	UserAgent       string
}

// Manager provides access to the NSE LAMA API.
type Manager struct {
	sync.RWMutex

	lo   *slog.Logger
	opts Opts

	client  *http.Client
	headers http.Header

	token string

	dbSeqID  int
	hwSeqID  int
	netSeqID int
	appSeqID int
	capSeqID int
}

type LoginReq struct {
	MemberID string `json:"memberId"`
	LoginID  string `json:"loginId"`
	Password string `json:"password"`
}

type LoginResp struct {
	Timestamp    int64  `json:"timestamp"`
	VersionNo    string `json:"versionNo"`
	MemberID     string `json:"memberId"`
	LoginID      string `json:"loginId"`
	ResponseCode int    `json:"responseCode"`
	ResponseDesc string `json:"responseDesc"`
	Token        string `json:"token"`
}

type MetricsResp struct {
	Timestamp    int64  `json:"timestamp"`
	VersionNo    string `json:"versionNo"`
	ResponseCode int    `json:"responseCode"`
	ResponseDesc string `json:"responseDesc"`
	Errors       []struct {
		ApplicationID int         `json:"applicationId"`
		ErrCode       int         `json:"errCode"`
		ErrDesc       string      `json:"errDesc"`
		ErrKey        string      `json:"errKey"`
		Measure       interface{} `json:"measure"`
	} `json:"errors"`
}

type MetricData struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type MetricValue struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
	Med float64 `json:"med"`
}

type MetricPayload struct {
	// -1=Not Applicable
	// 1= Client Connectivity
	// 2= Order Management System
	// 3= Risk Management System
	// 4= Exchange Connectivity
	// In our case, it's always 1.
	ApplicationID int          `json:"applicationId"`
	MetricData    []MetricData `json:"metricData"`
}

type HardwareReq struct {
	MemberID   string          `json:"memberId"`
	ExchangeID int             `json:"exchangeId"`
	SequenceID int             `json:"sequenceId"`
	LocationID int             `json:"locationId"`
	Timestamp  int64           `json:"timestamp"`
	Payload    []MetricPayload `json:"payload"`
}

type DatabaseReq struct {
	MemberID   string          `json:"memberId"`
	ExchangeID int             `json:"exchangeId"`
	SequenceID int             `json:"sequenceId"`
	LocationID int             `json:"locationId"`
	Timestamp  int64           `json:"timestamp"`
	Payload    []MetricPayload `json:"payload"`
}

type NetworkReq struct {
	MemberID   string          `json:"memberId"`
	ExchangeID int             `json:"exchangeId"`
	SequenceID int             `json:"sequenceId"`
	LocationID int             `json:"locationId"`
	Timestamp  int64           `json:"timestamp"`
	Payload    []MetricPayload `json:"payload"`
}

type AppReq struct {
	MemberID   string          `json:"memberId"`
	ExchangeID int             `json:"exchangeId"`
	SequenceID int             `json:"sequenceId"`
	LocationID int             `json:"locationId"`
	Timestamp  int64           `json:"timestamp"`
	Payload    []MetricPayload `json:"payload"`
}

// CapacityPayload represents the payload structure for capacity utilization metrics.
// The capacity metrics API uses a different payload structure compared to other metrics:
// - Other metrics (hardware, database, network, application) use an array of payload objects
// - Capacity metrics use a single object containing a metricData array
// This structure reflects the fact that capacity metrics are submitted once per day rather than every 5 minutes.
type CapacityPayload struct {
	MetricData []MetricData `json:"metricData"`
}

// CapacityReq represents the request structure for capacity utilization metrics.
// Uses "segment" field to specify market segment (Capital Markets, F&O, etc.).
type CapacityReq struct {
	MemberID   string          `json:"memberId"`
	ExchangeID int             `json:"exchangeId"`
	SequenceID int             `json:"sequenceId"`
	Segment    int             `json:"segment"` // Market segment identifier (1=Capital Markets, 2=F&O, etc.)
	Timestamp  int64           `json:"timestamp"`
	Payload    CapacityPayload `json:"payload"` // Single payload object for capacity metrics
}

func New(lo *slog.Logger, opts Opts) (*Manager, error) {
	client := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: opts.IdleConnTimeout,
		},
	}

	// Add common headers.
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Referer", opts.URL)
	h.Set("User-Agent", opts.UserAgent)
	h.Set("Accept-Language", "en-US")
	if strings.Contains(opts.URL, "uat") {
		h.Add("Cookie", "test")
	} else {
		h.Add("Cookie", "prod")
	}
	h.Set("Accept", "application/json") // Explicitly state accepted response format

	// Set common fields for logger.
	lgr := lo.With("login_id", opts.LoginID, "member_id", opts.MemberID, "exchange_id", opts.ExchangeID)
	lgr.Debug("mii-lama client created")

	mgr := &Manager{
		opts:     opts,
		lo:       lgr,
		client:   client,
		headers:  h,
		hwSeqID:  1,
		dbSeqID:  1,
		netSeqID: 1,
		capSeqID: 1,
	}

	return mgr, nil
}

// Login is used to generate a session token for further requests.
// Token is valid for 24 hours and after that it should be renewed again.
func (mgr *Manager) Login() error {
	endpoint := fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/auth/login")
	mgr.lo.Info("Starting login process", "URL", endpoint)

	loginPayload := LoginReq{
		MemberID: mgr.opts.MemberID,
		LoginID:  mgr.opts.LoginID,
		Password: mgr.opts.Password,
	}

	payload, err := json.Marshal(loginPayload)
	if err != nil {
		mgr.lo.Error("Unable to marshal login payload", "error", err)
		return fmt.Errorf("failed to marshal login payload: %v", err)
	}

	mgr.lo.Debug("Prepared login request payload", "payload", string(payload))

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		mgr.lo.Error("Unable to create HTTP request", "error", err)
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	resp, err := mgr.client.Do(req)
	if err != nil {
		mgr.lo.Error("HTTP request failed", "error", err)
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		mgr.lo.Error("Unexpected HTTP status code", "status_code", resp.StatusCode, "response_body", bodyString)
		return fmt.Errorf("HTTP request returned status code %d", resp.StatusCode)
	}

	var r LoginResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		mgr.lo.Error("Unable to unmarshal login response", "error", err)
		return fmt.Errorf("failed to unmarshal login response: %v", err)
	}

	if r.ResponseCode != NSE_RESP_CODE_SUCCESS {
		mgr.lo.Error("Login failed", "response_code", r.ResponseCode, "response_desc", r.ResponseDesc, "login_id", mgr.opts.LoginID, "member_id", mgr.opts.MemberID)
		return fmt.Errorf("login failed with NSE response code %d and description: %s", r.ResponseCode, r.ResponseDesc)
	}

	mgr.lo.Info("NSE login OK", "member_id", mgr.opts.MemberID)

	mgr.Lock()
	mgr.token = r.Token
	mgr.Unlock()

	return nil
}

// PushHWMetrics is used to push hardware metrics to NSE LAMA API.
func (mgr *Manager) PushHWMetrics(locationID int, host string, data models.HWPromResp) error {
	endpoint := fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/metrics/hardware")

	mgr.RLock()
	token := mgr.token
	seqID := mgr.hwSeqID
	mgr.RUnlock()

	hwPayload := createHardwareReq(data, mgr.opts.MemberID, mgr.opts.ExchangeID, seqID, locationID, DEFAULT_APPLICATION_ID)

	payload, err := json.Marshal(hwPayload)
	if err != nil {
		mgr.lo.Error("Failed to marshal hardware metrics payload", "error", err)
		return fmt.Errorf("failed to marshal hardware metrics payload: %v", err)
	}

	// Detailed request logging only in debug mode
	mgr.lo.Debug("Sending HW metrics", "host", host, "seq_id", hwPayload.SequenceID)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		mgr.lo.Error("Failed to create HTTP request", "error", err)
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	resp, err := mgr.client.Do(req)
	if err != nil {
		mgr.lo.Error("Hardware metrics HTTP request failed", "error", err)
		return fmt.Errorf("hardware metrics HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var r MetricsResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		mgr.lo.Error("Failed to unmarshal hardware metrics response", "error", err)
		return fmt.Errorf("failed to unmarshal hardware metrics response: %v", err)
	}

	// Success case - minimal logging
	if r.ResponseCode == NSE_RESP_CODE_SUCCESS || r.ResponseCode == NSE_RESP_CODE_PARTIAL_SUCCESS {
		mgr.lo.Debug("HW metrics OK", "host", host, "seq_id", hwPayload.SequenceID)
	}

	if resp.StatusCode != http.StatusOK {
		mgr.lo.Error("HW metrics failed", "code", r.ResponseCode, "desc", r.ResponseDesc)
		switch r.ResponseCode {
		case NSE_RESP_CODE_INVALID_TOKEN, NSE_RESP_CODE_EXPIRED_TOKEN:
			mgr.lo.Warn("Token expired, relogging")
			if err := mgr.Login(); err != nil {
				mgr.lo.Error("Relogin attempt failed", "error", err)
				return fmt.Errorf("failed to log in again: %v", err)
			}
			return fmt.Errorf("new token obtained after relogin, retrying hardware metrics push")

		case NSE_RESP_CODE_INVALID_SEQ_ID:
			return mgr.sequenceIDSyncHandler(r, "hardware")

		default:
			mgr.lo.Error("HW metrics failed", "response_code", r.ResponseCode)
			return fmt.Errorf("hardware metrics push failed with unhandled response code: %d", r.ResponseCode)
		}
	}

	if r.ResponseCode == NSE_RESP_CODE_SUCCESS || r.ResponseCode == NSE_RESP_CODE_PARTIAL_SUCCESS {
		mgr.Lock()
		mgr.hwSeqID++
		mgr.Unlock()
	}

	return nil
}

func (mgr *Manager) PushDBMetrics(locationID int, host string, data models.DBPromResp) error {
	endpoint := fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/metrics/database")

	mgr.RLock()
	token := mgr.token
	seqID := mgr.dbSeqID
	mgr.RUnlock()

	dbPayload := createDatabaseReq(data, mgr.opts.MemberID, mgr.opts.ExchangeID, seqID, locationID, DEFAULT_APPLICATION_ID)

	payload, err := json.Marshal(dbPayload)
	if err != nil {
		mgr.lo.Error("Failed to marshal database metrics payload", "error", err)
		return fmt.Errorf("failed to marshal database metrics payload: %v", err)
	}

	mgr.lo.Debug("Sending DB metrics", "host", host, "seq_id", dbPayload.SequenceID)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		mgr.lo.Error("Failed to create HTTP request", "error", err)
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	resp, err := mgr.client.Do(req)
	if err != nil {
		mgr.lo.Error("Database metrics HTTP request failed", "error", err)
		return fmt.Errorf("database metrics HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var r MetricsResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		mgr.lo.Error("Failed to unmarshal database metrics response", "error", err)
		return fmt.Errorf("failed to unmarshal database metrics response: %v", err)
	}

	mgr.lo.Debug("DB metrics response", "code", r.ResponseCode, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		mgr.lo.Error("DB metrics failed", "code", r.ResponseCode, "desc", r.ResponseDesc)
		switch r.ResponseCode {
		case NSE_RESP_CODE_INVALID_TOKEN, NSE_RESP_CODE_EXPIRED_TOKEN:
			mgr.lo.Warn("Token expired, relogging")
			if err := mgr.Login(); err != nil {
				mgr.lo.Error("Relogin attempt failed", "error", err)
				return fmt.Errorf("failed to log in again: %v", err)
			}
			return fmt.Errorf("new token obtained after relogin, retrying database metrics push")

		case NSE_RESP_CODE_INVALID_SEQ_ID:
			return mgr.sequenceIDSyncHandler(r, "database")

		default:
			mgr.lo.Error("DB metrics failed", "response_code", r.ResponseCode)
			return fmt.Errorf("database metrics push failed with unhandled response code: %d", r.ResponseCode)
		}
	}

	if r.ResponseCode == NSE_RESP_CODE_SUCCESS || r.ResponseCode == NSE_RESP_CODE_PARTIAL_SUCCESS {
		mgr.Lock()
		mgr.dbSeqID++
		mgr.Unlock()
	}

	return nil
}

// PushNetworkMetrics sends network metrics to NSE LAMA API.
func (mgr *Manager) PushNetworkMetrics(locationID int, host string, data models.NetworkPromResp) error {
	endpoint := fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/metrics/network")

	mgr.RLock()
	token := mgr.token
	seqID := mgr.netSeqID
	mgr.RUnlock()

	netPayload := createNetworkReq(data, mgr.opts.MemberID, mgr.opts.ExchangeID, seqID, locationID, DEFAULT_APPLICATION_ID)

	payload, err := json.Marshal(netPayload)
	if err != nil {
		mgr.lo.Error("Failed to marshal network metrics payload", "error", err)
		return fmt.Errorf("failed to marshal network metrics payload: %v", err)
	}

	mgr.lo.Debug("Sending NET metrics", "host", host, "seq_id", netPayload.SequenceID)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		mgr.lo.Error("Failed to create HTTP request", "error", err)
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	resp, err := mgr.client.Do(req)
	if err != nil {
		mgr.lo.Error("Network metrics HTTP request failed", "error", err)
		return fmt.Errorf("network metrics HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var r MetricsResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		mgr.lo.Error("Failed to unmarshal network metrics response", "error", err)
		return fmt.Errorf("failed to unmarshal network metrics response: %v", err)
	}

	mgr.lo.Debug("NET metrics response", "code", r.ResponseCode, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		mgr.lo.Error("NET metrics failed", "code", r.ResponseCode, "desc", r.ResponseDesc)
		switch r.ResponseCode {
		case NSE_RESP_CODE_INVALID_TOKEN, NSE_RESP_CODE_EXPIRED_TOKEN:
			mgr.lo.Warn("Token expired, relogging")
			if err := mgr.Login(); err != nil {
				mgr.lo.Error("Relogin attempt failed", "error", err)
				return fmt.Errorf("failed to log in again: %v", err)
			}
			return fmt.Errorf("new token obtained after relogin, retrying network metrics push")

		case NSE_RESP_CODE_INVALID_SEQ_ID:
			return mgr.sequenceIDSyncHandler(r, "network")

		default:
			mgr.lo.Error("NET metrics failed", "response_code", r.ResponseCode)
			return fmt.Errorf("network metrics push failed with unhandled response code: %d", r.ResponseCode)
		}
	}

	if r.ResponseCode == NSE_RESP_CODE_SUCCESS || r.ResponseCode == NSE_RESP_CODE_PARTIAL_SUCCESS {
		mgr.Lock()
		mgr.netSeqID++
		mgr.Unlock()
	}

	return nil
}

// PushAppMetrics sends app metrics to NSE LAMA API.
func (mgr *Manager) PushAppMetrics(locationID int, host string, data models.AppPromResp) error {
	endpoint := fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/metrics/application")

	mgr.RLock()
	token := mgr.token
	seqID := mgr.appSeqID
	mgr.RUnlock()

	appPayload := createAppReq(data, mgr.opts.MemberID, mgr.opts.ExchangeID, seqID, locationID, DEFAULT_APPLICATION_ID)

	payload, err := json.Marshal(appPayload)
	if err != nil {
		mgr.lo.Error("Failed to marshal app metrics payload", "error", err)
		return fmt.Errorf("failed to marshal app metrics payload: %v", err)
	}

	mgr.lo.Debug("Sending APP metrics", "host", host, "seq_id", appPayload.SequenceID)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		mgr.lo.Error("Failed to create HTTP request", "error", err)
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	resp, err := mgr.client.Do(req)
	if err != nil {
		mgr.lo.Error("App metrics HTTP request failed", "error", err)
		return fmt.Errorf("app metrics HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var r MetricsResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		mgr.lo.Error("Failed to unmarshal app metrics response", "error", err)
		return fmt.Errorf("failed to unmarshal app metrics response: %v", err)
	}

	mgr.lo.Debug("APP metrics response", "code", r.ResponseCode, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		mgr.lo.Error("APP metrics failed", "code", r.ResponseCode, "desc", r.ResponseDesc)
		switch r.ResponseCode {
		case NSE_RESP_CODE_INVALID_TOKEN, NSE_RESP_CODE_EXPIRED_TOKEN:
			mgr.lo.Warn("Token expired, relogging")
			if err := mgr.Login(); err != nil {
				mgr.lo.Error("Relogin attempt failed", "error", err)
				return fmt.Errorf("failed to log in again: %v", err)
			}
			return fmt.Errorf("new token obtained after relogin, retrying app metrics push")

		case NSE_RESP_CODE_INVALID_SEQ_ID:
			return mgr.sequenceIDSyncHandler(r, "application")

		default:
			mgr.lo.Error("APP metrics failed", "response_code", r.ResponseCode)
			return fmt.Errorf("app metrics push failed with unhandled response code: %d", r.ResponseCode)
		}
	}

	if r.ResponseCode == NSE_RESP_CODE_SUCCESS || r.ResponseCode == NSE_RESP_CODE_PARTIAL_SUCCESS {
		mgr.Lock()
		mgr.appSeqID++
		mgr.Unlock()
	}

	return nil
}

func createNetworkReq(metrics models.NetworkPromResp, memberId string, exchangeId, sequenceId, locationID, applicationId int) NetworkReq {
	return NetworkReq{
		MemberID:   memberId,
		ExchangeID: exchangeId,
		SequenceID: sequenceId,
		LocationID: locationID,
		Timestamp:  time.Now().Unix(),
		Payload: []MetricPayload{
			{
				ApplicationID: applicationId,
				MetricData: []MetricData{
					newMetricData("packetCount", float64(metrics.PacketErrors), true),
					newMetricData("bandwidth", 0.0, false),
				},
			},
		},
	}
}

func createAppReq(metrics models.AppPromResp, memberId string, exchangeId, sequenceId, locationID, applicationId int) AppReq {
	return AppReq{
		MemberID:   memberId,
		ExchangeID: exchangeId,
		SequenceID: sequenceId,
		LocationID: locationID,
		Timestamp:  time.Now().Unix(),
		Payload: []MetricPayload{
			{
				ApplicationID: applicationId,
				MetricData: []MetricData{
					newMetricData("throughput", float64(metrics.Throughput), false),
					newMetricData("failureTradeApi", float64(metrics.FailureCount), true),
					newMetricData("latency", float64(metrics.Latency), false),
					newMetricData("failureAuthentication", float64(metrics.FailureAuth), true),
				},
			},
		},
	}
}

func createHardwareReq(metrics models.HWPromResp, memberId string, exchangeId, sequenceId, locationID, applicationId int) HardwareReq {
	return HardwareReq{
		MemberID:   memberId,
		ExchangeID: exchangeId,
		SequenceID: sequenceId,
		LocationID: locationID,
		Timestamp:  time.Now().Unix(),
		Payload: []MetricPayload{
			{
				ApplicationID: applicationId,
				MetricData: []MetricData{
					newMetricData("cpu", metrics.CPU, false),
					newMetricData("memory", metrics.Mem, false),
					newMetricData("disk", metrics.Disk, false),
					newMetricData("uptime", metrics.Uptime, false),
				},
			},
		},
	}
}

func createDatabaseReq(metrics models.DBPromResp, memberId string, exchangeId, sequenceId, locationID, applicationId int) DatabaseReq {
	return DatabaseReq{
		MemberID:   memberId,
		ExchangeID: exchangeId,
		SequenceID: sequenceId,
		LocationID: locationID,
		Timestamp:  time.Now().Unix(),
		Payload: []MetricPayload{
			{
				ApplicationID: applicationId,
				MetricData: []MetricData{
					newMetricData("status", float64(metrics.Status), true),
					newMetricData("latency", 0.0, false),
					newMetricData("qSize", 0.0, false),
					newMetricData("bandwidth", 0.0, false),
				},
			},
		},
	}
}

// extractExpectedSequenceID extracts the expected SequenceID value from a provided
// error description. It returns the extracted SequenceID as an integer. If the
// description does not contain a valid SequenceID, the function returns an error.

func extractExpectedSequenceID(desc string) (int, error) {
	matches := sequenceIDRegex.FindStringSubmatch(desc)

	if len(matches) < 2 {
		return 0, fmt.Errorf("expected SequenceID not found in description: '%s'", desc)
	}

	return strconv.Atoi(matches[1])
}

// Function to create a new MetricData.
func newMetricData(key string, avg float64, simple bool) MetricData {
	var value interface{}
	if simple {
		value = avg
	} else {
		var strValue string
		switch key {
		case "uptime":
			strValue = fmt.Sprintf("%.0f", avg)
		default:
			strValue = fmt.Sprintf("%.2f", avg)
		}

		// Convert the string back to a float64.
		data, err := strconv.ParseFloat(strValue, 64)
		if err != nil {
			// Log error and fallback to original value
			// Note: This should rarely happen as we control the formatting
			data = avg
		}

		value = MetricValue{
			Min: data,
			Max: data,
			Avg: data,
			Med: data,
		}
	}

	return MetricData{
		Key:   key,
		Value: value,
	}
}

// PushCapacityMetrics sends capacity utilization metrics to NSE LAMA API.
func (mgr *Manager) PushCapacityMetrics(locationID int, host string, data models.CapacityPromResp, benchmarkCapacity float64, segmentID int) error {
	endpoint := fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/metrics/cap-utilization")

	mgr.RLock()
	token := mgr.token
	seqID := mgr.capSeqID
	mgr.RUnlock()

	// Use the provided segment ID
	capacityPayload := createCapacityReq(data, mgr.opts.MemberID, mgr.opts.ExchangeID, seqID, segmentID, benchmarkCapacity)

	payload, err := json.Marshal(capacityPayload)
	if err != nil {
		mgr.lo.Error("Failed to marshal capacity metrics payload", "error", err)
		return fmt.Errorf("failed to marshal capacity metrics payload: %v", err)
	}

	mgr.lo.Debug("Sending CAP metrics", "host", host, "seq_id", capacityPayload.SequenceID)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		mgr.lo.Error("Failed to create HTTP request", "error", err)
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	resp, err := mgr.client.Do(req)
	if err != nil {
		mgr.lo.Error("Capacity metrics HTTP request failed", "error", err)
		return fmt.Errorf("capacity metrics HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the raw response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		mgr.lo.Error("Failed to read capacity metrics response body", "error", err)
		return fmt.Errorf("failed to read capacity metrics response body: %v", err)
	}

	var r MetricsResp
	if err := json.Unmarshal(bodyBytes, &r); err != nil {
		mgr.lo.Error("Failed to unmarshal capacity metrics response", "error", err, "raw_response", string(bodyBytes))
		return fmt.Errorf("failed to unmarshal capacity metrics response: %v", err)
	}

	mgr.lo.Debug("CAP metrics response", "code", r.ResponseCode, "status", resp.StatusCode)

	// Handle NSE response codes regardless of HTTP status (NSE sends errors as HTTP 500 with JSON body)
	// Note: Capacity metrics API returns responseCode 200 for success (different from other APIs that use 601)
	if resp.StatusCode != http.StatusOK || (r.ResponseCode != NSE_RESP_CODE_SUCCESS && r.ResponseCode != 200) {
		mgr.lo.Error("CAP metrics failed", "code", r.ResponseCode, "desc", r.ResponseDesc)
		switch r.ResponseCode {
		case NSE_RESP_CODE_INVALID_TOKEN, NSE_RESP_CODE_EXPIRED_TOKEN:
			mgr.lo.Warn("Token expired, relogging")
			if err := mgr.Login(); err != nil {
				mgr.lo.Error("Relogin attempt failed", "error", err)
				return fmt.Errorf("failed to log in again: %v", err)
			}
			return fmt.Errorf("new token obtained after relogin, retrying capacity metrics push")

		case NSE_RESP_CODE_INVALID_SEQ_ID:
			return mgr.sequenceIDSyncHandler(r, "capacity")

		case NSE_RESP_CODE_INVALID_REQUEST:
			roundedOrders := math.Round(data.OrdersCount)
			mgr.lo.Error("CAP metrics validation failed", "response_code", r.ResponseCode, "desc", r.ResponseDesc, "segment", segmentID, "orders_count", data.OrdersCount, "rounded_orders", roundedOrders, "benchmark", benchmarkCapacity)
			return fmt.Errorf("capacity metrics request invalid: %s", r.ResponseDesc)

		default:
			mgr.lo.Error("CAP metrics failed", "response_code", r.ResponseCode)
			return fmt.Errorf("capacity metrics push failed with unhandled response code: %d", r.ResponseCode)
		}
	}

	// Increment sequence ID on successful push
	// Capacity API uses responseCode 200 for success (different from other APIs that use 601/602)
	if r.ResponseCode == NSE_RESP_CODE_SUCCESS || r.ResponseCode == NSE_RESP_CODE_PARTIAL_SUCCESS || r.ResponseCode == 200 {
		mgr.Lock()
		mgr.capSeqID++
		mgr.Unlock()
	}

	return nil
}

// createCapacityReq creates a capacity utilization request for NSE's capacity metrics API.
// Capacity metrics track peak order performance against benchmark capacity and use:
// - "peakOrder" for maximum orders per second achieved
// - "benchmark" for installed capacity limit
// Values are simple numbers rather than statistical objects (min/max/avg/med) used by other metrics.
// Segment ID maps to market segments: 1=Capital Markets, 2=F&O, 3=Currency Derivatives, 4=Commodity
func createCapacityReq(metrics models.CapacityPromResp, memberId string, exchangeId, sequenceId, segmentId int, benchmarkCapacity float64) CapacityReq {
	// Round orders count to nearest integer as required by API
	roundedOrdersCount := math.Round(metrics.OrdersCount)

	return CapacityReq{
		MemberID:   memberId,
		ExchangeID: exchangeId,
		SequenceID: sequenceId,
		Segment:    segmentId, // Market segment for capacity tracking
		Timestamp:  time.Now().Unix(),
		Payload: CapacityPayload{
			MetricData: []MetricData{
				newMetricData("benchmark", benchmarkCapacity, true),  // Installed capacity benchmark (first)
				newMetricData("peakOrder", roundedOrdersCount, true), // Peak orders per second achieved (second, rounded)
			},
		},
	}
}
