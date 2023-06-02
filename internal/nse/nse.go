package nse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	USER_AGENT                    = "mii-lama"
	NSE_RESP_CODE_SUCCESS         = 601
	NSE_RESP_CODE_PARTIAL_SUCCESS = 602
	NSE_RESP_CODE_INVALID_LOGIN   = 701
	NSE_RESP_CODE_INVALID_SEQ_ID  = 704
	NSE_RESP_CODE_INVALID_TOKEN   = 801
	NSE_RESP_CODE_EXPIRED_TOKEN   = 802
)

type Opts struct {
	URL             string
	LoginID         string
	MemberID        string
	ExchangeID      int
	Password        string
	Timeout         time.Duration
	IdleConnTimeout time.Duration
}

// Manager provides access to the NSE LAMA API.
type Manager struct {
	sync.RWMutex

	lo   *slog.Logger
	opts Opts

	client  *http.Client
	headers http.Header

	token string
	seqID int
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
	h.Set("User-Agent", USER_AGENT)
	if strings.Contains(opts.URL, "uat") {
		h.Add("Cookie", "test")
	}

	// Set common fields for logger.
	lgr := lo.With("login_id", opts.LoginID, "member_id", opts.MemberID, "exchange_id", opts.ExchangeID)
	lgr.Debug("mii-lama client created")

	mgr := &Manager{
		opts:    opts,
		lo:      lgr,
		client:  client,
		headers: h,
		seqID:   1,
	}

	return mgr, nil
}

// Login is used to generate a session token for further requests.
// Token is valid for 24 hours and after that it should be renewed again.
func (mgr *Manager) Login() error {
	var (
		endpoint = fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/auth/login")
	)
	mgr.lo.Info("attempting login", "url", endpoint)

	loginPayload := LoginReq{
		MemberID: mgr.opts.MemberID,
		LoginID:  mgr.opts.LoginID,
		Password: mgr.opts.Password,
	}

	payload, err := json.Marshal(loginPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal login payload: %w", err)
	}

	mgr.lo.Debug("sending login request", "payload", string(payload))

	// Create a new request using http
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	// If an error occurred while creating the request, handle it
	if err != nil {
		return fmt.Errorf("could not create request: %v", err)
	}

	// Add headers to the request.
	req.Header.Set("Content-Type", "application/json")
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	// Send the request via the client
	resp, err := mgr.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request for login failed: %w", err)
	}
	defer resp.Body.Close()

	// Check the status code of the response.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed with status code: %d", resp.StatusCode)
	}

	// Decode the response directly into LoginResp struct
	var r LoginResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("failed to unmarshal login response: %w", err)
	}

	// Check if login was successful.
	if r.ResponseCode != NSE_RESP_CODE_SUCCESS {
		mgr.lo.Error("login failed", "response_code", r.ResponseCode, "response_desc", r.ResponseDesc, "login_id", mgr.opts.LoginID, "member_id", mgr.opts.MemberID)
		return fmt.Errorf("login failed with nse response code: %d", r.ResponseCode)
	}

	mgr.lo.Debug("login successful", "login_id", mgr.opts.LoginID, "member_id", mgr.opts.MemberID, "token", r.Token)

	// Save token for future use.
	mgr.Lock()
	mgr.token = r.Token
	mgr.Unlock()

	return nil
}

func (mgr *Manager) PushHWMetrics(host string, data models.HWPromResp) error {
	var (
		endpoint = fmt.Sprintf("%s%s", mgr.opts.URL, "/api/V1/metrics/hardware")
	)
	// Get token and sequence id.
	mgr.RLock()
	token := mgr.token
	seqID := mgr.seqID
	mgr.RUnlock()

	hwPayload := createHardwareReq(data, mgr.opts.MemberID, mgr.opts.ExchangeID, seqID, 1)

	payload, err := json.Marshal(hwPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal hw metrics payload: %w", err)
	}

	mgr.lo.Info("attempting to send hw metrics", "host", host, "url", endpoint, "payload", string(payload), "headers", mgr.headers)

	// Create a new request.
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	// If an error occurred while creating the request, handle it
	if err != nil {
		return fmt.Errorf("could not create request: %v", err)
	}

	// Add headers to the request
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	for k, v := range mgr.headers {
		req.Header.Set(k, strings.Join(v, ","))
	}

	// // Dump the request for debugging.
	// requestDump, err := httputil.DumpRequestOut(req, true)
	// if err != nil {
	// 	fmt.Println("Error dumping request:", err)
	// } else {
	// 	fmt.Println("Request:", string(requestDump))
	// }

	// Send the request via the client.
	resp, err := mgr.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request for hw metrics failed: %w", err)
	}
	defer resp.Body.Close()

	var r MetricsResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("failed to unmarshal hw metrics response: %w", err)
	}

	mgr.lo.Info("metrics push response", "response_code", r.ResponseCode, "response_desc", r.ResponseDesc, "http_status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		mgr.lo.Error("metrics push failed", "response_code", r.ResponseCode, "response_desc", r.ResponseDesc, "errors", r.Errors)
		fmt.Println(r.ResponseDesc, r.Errors, r.ResponseCode)
		switch r.ResponseCode {
		// Handle case where token is invalid.
		case NSE_RESP_CODE_INVALID_TOKEN, NSE_RESP_CODE_EXPIRED_TOKEN:
			mgr.lo.Error("token is invalid, attempting to login")
			if err := mgr.Login(); err != nil {
				return fmt.Errorf("failed to relogin: %w", err)
			}
			// Retry push after login.
			return fmt.Errorf("new token is set after login, retry again")

		// Handle case where sequence id is invalid.
		case NSE_RESP_CODE_INVALID_SEQ_ID:
			mgr.lo.Warn("sequence id is invalid, attempting to update")
			// Extract expected sequence id from error description.
			expectedSeqID, err := extractExpectedSequenceID(r.ResponseDesc)
			if err != nil {
				return fmt.Errorf("failed to extract expected sequence id: %w", err)
			}
			mgr.lo.Debug("expected sequence id", "expected_seq_id", expectedSeqID)
			mgr.Lock()
			mgr.seqID = expectedSeqID
			mgr.Unlock()
			return fmt.Errorf("sequence id is updated, retry again")

		default:
			return fmt.Errorf("metrics push failed with nse response code: %d", r.ResponseCode)
		}
	}

	// Increment sequence id on successful/partial push.
	if r.ResponseCode == NSE_RESP_CODE_SUCCESS || r.ResponseCode == NSE_RESP_CODE_PARTIAL_SUCCESS {
		mgr.Lock()
		mgr.seqID++
		mgr.Unlock()
	}

	return nil
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
	Key   string `json:"key"`
	Value struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
		Avg float64 `json:"avg"`
		Med float64 `json:"med"`
	} `json:"value"`
}

type MetricPayload struct {
	ApplicationID int          `json:"applicationId"`
	MetricData    []MetricData `json:"metricData"`
}

type HardwareReq struct {
	MemberID   string          `json:"memberId"`
	ExchangeID int             `json:"exchangeId"`
	SequenceID int             `json:"sequenceId"`
	Timestamp  int64           `json:"timestamp"`
	Payload    []MetricPayload `json:"payload"`
}

// Function to create a new MetricData
func newMetricData(key string, avg float64) MetricData {
	var value string
	switch key {
	case "uptime":
		value = fmt.Sprintf("%.0f", avg)
	default:
		value = fmt.Sprintf("%.2f", avg)
	}

	// Convert the string back to a float64.
	data, err := strconv.ParseFloat(value, 64)
	if err != nil {
		// TODO: Handle error. For now fallback to original value.
		fmt.Println("failed to convert string to float64", "value", value, "error", err, "key", key, "avg", avg)
		data = avg
	}

	return MetricData{
		Key: key,
		Value: struct {
			Min float64 `json:"min"`
			Max float64 `json:"max"`
			Avg float64 `json:"avg"`
			Med float64 `json:"med"`
		}{
			Min: 0,
			Max: 0,
			Avg: data,
			Med: 0,
		},
	}
}

// Function to create a new HardwareReq
func createHardwareReq(metrics models.HWPromResp, memberId string, exchangeId, sequenceId, applicationId int) HardwareReq {
	return HardwareReq{
		MemberID:   memberId,
		ExchangeID: exchangeId,
		SequenceID: sequenceId,
		Timestamp:  time.Now().Unix(),
		Payload: []MetricPayload{
			{
				ApplicationID: applicationId,
				MetricData: []MetricData{
					newMetricData("cpu", metrics.CPU),
					newMetricData("memory", metrics.Mem),
					newMetricData("disk", metrics.Disk),
					newMetricData("uptime", metrics.Uptime),
				},
			},
		},
	}
}

// extractExpectedSequenceID extracts the expected SequenceID value from a provided
// error description. It returns the extracted SequenceID as an integer. If the
// description does not contain a valid SequenceID, the function returns an error.
func extractExpectedSequenceID(desc string) (int, error) {
	re := regexp.MustCompile(`SequenceId should be (\d+)`)
	matches := re.FindStringSubmatch(desc)

	if len(matches) < 2 {
		return 0, errors.New("expected SequenceID not found in the description")
	}

	return strconv.Atoi(matches[1])
}
