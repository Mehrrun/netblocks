package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
)

// TrafficMonitor monitors Iran's internet traffic using Cloudflare Radar API
type TrafficMonitor struct {
	client           *http.Client
	lastUpdate       time.Time
	cachedData       *TrafficData
	mu               sync.RWMutex
	baseline         float64
	cloudflareToken  string  // API Token (preferred)
	cloudflareEmail  string  // Legacy: API Key email
	cloudflareKey    string  // Legacy: API Key
}

// TrafficData represents Iran's internet traffic statistics
type TrafficData struct {
	CurrentLevel  float64
	Trend24h      []float64
	Timestamps    []time.Time
	ChangePercent float64
	Status        string
	StatusEmoji   string
	LastUpdate    time.Time
}

// CloudflareRadarResponse represents the API response
type CloudflareRadarResponse struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// NewTrafficMonitor creates a new traffic monitor
// Accepts either API Token (cloudflareToken) or API Key (cloudflareEmail + cloudflareKey)
// API Token is preferred for security
func NewTrafficMonitor(cloudflareToken, cloudflareEmail, cloudflareKey string) *TrafficMonitor {
	log.Printf("NewTrafficMonitor: token set=%v (len=%d), email set=%v, key set=%v", 
		cloudflareToken != "", len(cloudflareToken),
		cloudflareEmail != "", cloudflareKey != "")
	
	return &TrafficMonitor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseline:        100.0, // Will be calculated from data
		cloudflareToken: cloudflareToken,
		cloudflareEmail: cloudflareEmail,
		cloudflareKey:   cloudflareKey,
	}
}

// GetTrafficData returns cached or fresh traffic data
func (tm *TrafficMonitor) GetTrafficData(ctx context.Context) (*TrafficData, error) {
	tm.mu.RLock()
	// Return cached data if fresh (less than 5 minutes old)
	if tm.cachedData != nil && time.Since(tm.lastUpdate) < 5*time.Minute {
		data := tm.cachedData
		tm.mu.RUnlock()
		return data, nil
	}
	tm.mu.RUnlock()

	// Fetch fresh data
	return tm.FetchFromCloudflare(ctx)
}

// FetchFromCloudflare fetches traffic data from Cloudflare Radar API
func (tm *TrafficMonitor) FetchFromCloudflare(ctx context.Context) (*TrafficData, error) {
	// Cloudflare Radar API endpoint for Iran HTTP traffic bandwidth
	// Using timeseries endpoint - returns HTTP request volume/time over time.
	// Request 7d to maximize data availability, then slice last 24h locally.
	// The correct endpoint is /radar/http/timeseries (NOT timeseries_groups).
	// dateRange: valid values are "1d", "7d", "14d", "24h", etc.
	// location: IR for Iran (fallback to IRN if IR returns no data)
	// aggInterval: aggregation interval like "1h", "1d", etc.
	url := "https://api.cloudflare.com/client/v4/radar/http/timeseries?location=IR&dateRange=7d&aggInterval=1h&format=json"

	log.Printf("Fetching Cloudflare Radar data from: %s", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return nil, err
	}

	req.Header.Set("User-Agent", "NetBlocks-Monitor/1.0")
	
	// Add Cloudflare authentication headers
	authMethod := "none"
	if tm.cloudflareToken != "" {
		req.Header.Set("Authorization", "Bearer "+tm.cloudflareToken)
		authMethod = "Bearer Token"
		log.Printf("Using Cloudflare Bearer Token authentication (token length: %d)", len(tm.cloudflareToken))
	} else if tm.cloudflareEmail != "" && tm.cloudflareKey != "" {
		req.Header.Set("X-Auth-Email", tm.cloudflareEmail)
		req.Header.Set("X-Auth-Key", tm.cloudflareKey)
		authMethod = "API Key"
		log.Printf("Using Cloudflare API Key authentication (email: %s)", tm.cloudflareEmail)
	} else {
		log.Printf("WARNING: No Cloudflare credentials available - request will likely fail")
	}

	resp, err := tm.client.Do(req)
	if err != nil {
		log.Printf("Error making HTTP request to Cloudflare: %v (auth method: %s)", err, authMethod)
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body first (even if error) to see what Cloudflare says
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	log.Printf("Cloudflare API response: Status %d %s (auth method: %s)", resp.StatusCode, resp.Status, authMethod)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Cloudflare API returned non-200 status. Response body: %s", string(bodyBytes))
		
		// Try to parse error response
		var errorResp struct {
			Success bool `json:"success"`
			Errors  []struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &errorResp); jsonErr == nil && len(errorResp.Errors) > 0 {
			for _, err := range errorResp.Errors {
				log.Printf("Cloudflare API error %d: %s", err.Code, err.Message)
			}
		}
		
		return nil, fmt.Errorf("cloudflare API status %d", resp.StatusCode)
	}

	var apiResp CloudflareRadarResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		log.Printf("Error decoding JSON response: %v", err)
		log.Printf("Response body (first 500 chars): %s", string(bodyBytes[:min(500, len(bodyBytes))]))
		return nil, err
	}

	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			log.Printf("Cloudflare API returned success=false with errors:")
			for _, err := range apiResp.Errors {
				log.Printf("  Error %d: %s", err.Code, err.Message)
			}
		} else {
			log.Printf("Cloudflare API returned success=false (no error details provided)")
		}
		return nil, fmt.Errorf("cloudflare API returned success=false")
	}

	timestamps, values, found := extractSeries(apiResp.Result)
	if !found || len(values) == 0 {
		// Retry with IRN location (some Radar datasets use ISO3)
		retryURL := "https://api.cloudflare.com/client/v4/radar/http/timeseries?location=IRN&dateRange=7d&aggInterval=1h&format=json"
		log.Printf("Cloudflare API returned empty data for IR, retrying with IRN: %s", retryURL)
		retryData, ok := tm.fetchWithURL(ctx, retryURL)
		if ok {
			return retryData, nil
		}

		log.Printf("Cloudflare API returned empty or unrecognized data structure")
		log.Printf("Full response body (first 2000 chars): %s", string(bodyBytes[:min(2000, len(bodyBytes))]))
		return nil, fmt.Errorf("no traffic data in response")
	}

	// Keep only the last 24 data points (24 hours) to match chart expectations
	timestamps, values = sliceLast24(timestamps, values)
	log.Printf("Cloudflare API success - received %d data points (last 24h)", len(values))

	// Process the data
	data, err := tm.processData(values, timestamps)
	if err != nil {
		log.Printf("Error processing traffic data: %v", err)
		return nil, err
	}

	log.Printf("Traffic data processed successfully - Current Level: %.1f%%, Status: %s %s", 
		data.CurrentLevel, data.StatusEmoji, data.Status)

	// Cache the data
	tm.mu.Lock()
	tm.cachedData = data
	tm.lastUpdate = time.Now()
	tm.mu.Unlock()

	return data, nil
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getKeys returns all keys from a map (for debugging)
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

type radarSerie struct {
	Timestamps []string  `json:"timestamps"`
	Values     []float64 `json:"values"`
}

type radarResult struct {
	Serie0    *radarSerie  `json:"serie_0"`
	Serie0Alt *radarSerie  `json:"serie0"`
	Series    []radarSerie `json:"series"`
	Data      *radarSerie  `json:"data"`
	Timeseries []radarSerie `json:"timeseries"`
	// Some responses return timestamps/values directly under result
	Timestamps []string  `json:"timestamps"`
	Values     []float64 `json:"values"`
}

func extractSeries(resultRaw json.RawMessage) ([]string, []float64, bool) {
	var rr radarResult
	if err := json.Unmarshal(resultRaw, &rr); err == nil {
		if len(rr.Values) > 0 && len(rr.Timestamps) > 0 {
			return rr.Timestamps, rr.Values, true
		}
		if rr.Serie0 != nil && len(rr.Serie0.Values) > 0 {
			return rr.Serie0.Timestamps, rr.Serie0.Values, true
		}
		if rr.Serie0Alt != nil && len(rr.Serie0Alt.Values) > 0 {
			return rr.Serie0Alt.Timestamps, rr.Serie0Alt.Values, true
		}
		if len(rr.Series) > 0 && len(rr.Series[0].Values) > 0 {
			return rr.Series[0].Timestamps, rr.Series[0].Values, true
		}
		if rr.Data != nil && len(rr.Data.Values) > 0 {
			return rr.Data.Timestamps, rr.Data.Values, true
		}
		if len(rr.Timeseries) > 0 && len(rr.Timeseries[0].Values) > 0 {
			return rr.Timeseries[0].Timestamps, rr.Timeseries[0].Values, true
		}
	}

	// Try direct serie object at result root
	var direct radarSerie
	if err := json.Unmarshal(resultRaw, &direct); err == nil && len(direct.Values) > 0 {
		return direct.Timestamps, direct.Values, true
	}

	var raw map[string]interface{}
	if json.Unmarshal(resultRaw, &raw) != nil {
		return nil, nil, false
	}

	// Try common keys in generic map
	for _, key := range []string{"timestamps", "values", "serie_0", "serie0", "series", "data", "timeseries"} {
		if v, ok := raw[key]; ok {
			if key == "timestamps" || key == "values" {
				// If timestamps/values are at the root, parse as map
				if ts, vals, ok := parseSerie(raw); ok {
					return ts, vals, true
				}
			}
			if ts, vals, ok := parseSerie(v); ok {
				return ts, vals, true
			}
		}
	}

	return nil, nil, false
}

func parseSerie(v interface{}) ([]string, []float64, bool) {
	switch s := v.(type) {
	case map[string]interface{}:
		timestamps := toStringSlice(s["timestamps"])
		values := toFloatSlice(s["values"])
		if len(values) > 0 && len(timestamps) > 0 {
			return timestamps, values, true
		}
		// Some responses may use "value" or "data" with pairs/objects
		if len(values) == 0 {
			values = toFloatSlice(s["value"])
		}
		if len(values) == 0 {
			if ts, vals, ok := parseSeriesPairs(s["data"]); ok {
				return ts, vals, true
			}
		}
		// Some responses may use a map of named series
		for _, item := range s {
			if ts, vals, ok := parseSerie(item); ok {
				return ts, vals, true
			}
		}
		// If values exist but timestamps are missing, accept and generate timestamps later
		if len(values) > 0 && len(timestamps) == 0 {
			return nil, values, true
		}
	case []interface{}:
		if len(s) > 0 {
			return parseSerie(s[0])
		}
	}
	return nil, nil, false
}

func toStringSlice(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
			continue
		}
		if ts, ok := normalizeTimestamp(item); ok {
			out = append(out, ts)
		}
	}
	return out
}

func toFloatSlice(v interface{}) []float64 {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]float64, 0, len(raw))
	for _, item := range raw {
		if f, ok := toFloat(item); ok {
			out = append(out, f)
		}
	}
	return out
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f, true
		}
		return 0, false
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func normalizeTimestamp(v interface{}) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		return time.Unix(int64(t), 0).UTC().Format(time.RFC3339), true
	case int:
		return time.Unix(int64(t), 0).UTC().Format(time.RFC3339), true
	case int64:
		return time.Unix(t, 0).UTC().Format(time.RFC3339), true
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return time.Unix(int64(f), 0).UTC().Format(time.RFC3339), true
		}
	}
	return "", false
}

func parseSeriesPairs(v interface{}) ([]string, []float64, bool) {
	raw, ok := v.([]interface{})
	if !ok || len(raw) == 0 {
		return nil, nil, false
	}

	timestamps := make([]string, 0, len(raw))
	values := make([]float64, 0, len(raw))

	for _, item := range raw {
		switch row := item.(type) {
		case []interface{}:
			if len(row) < 2 {
				continue
			}
			ts, okTs := normalizeTimestamp(row[0])
			val, okVal := toFloat(row[1])
			if okTs && okVal {
				timestamps = append(timestamps, ts)
				values = append(values, val)
			}
		case map[string]interface{}:
			ts, okTs := normalizeTimestamp(firstOf(row, "timestamp", "ts", "date", "datetime", "time"))
			val, okVal := toFloat(firstOf(row, "value", "val", "y"))
			if okTs && okVal {
				timestamps = append(timestamps, ts)
				values = append(values, val)
			}
		}
	}

	if len(values) == 0 || len(timestamps) == 0 {
		return nil, nil, false
	}

	return timestamps, values, true
}

func firstOf(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func sliceLast24(timestamps []string, values []float64) ([]string, []float64) {
	if len(values) <= 24 || len(timestamps) <= 24 {
		return timestamps, values
	}
	start := len(values) - 24
	if len(timestamps) > start {
		return timestamps[start:], values[start:]
	}
	return timestamps, values[start:]
}

// fetchWithURL fetches and parses Radar data using a specific URL.
// Returns data and true if successful, otherwise nil,false.
func (tm *TrafficMonitor) fetchWithURL(ctx context.Context, url string) (*TrafficData, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "NetBlocks-Monitor/1.0")
	if tm.cloudflareToken != "" {
		req.Header.Set("Authorization", "Bearer "+tm.cloudflareToken)
	} else if tm.cloudflareEmail != "" && tm.cloudflareKey != "" {
		req.Header.Set("X-Auth-Email", tm.cloudflareEmail)
		req.Header.Set("X-Auth-Key", tm.cloudflareKey)
	}

	resp, err := tm.client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	var apiResp CloudflareRadarResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil || !apiResp.Success {
		return nil, false
	}

	ts, vals, found := extractSeries(apiResp.Result)
	if !found || len(vals) == 0 {
		return nil, false
	}

	ts, vals = sliceLast24(ts, vals)
	data, err := tm.processData(vals, ts)
	if err != nil {
		return nil, false
	}
	return data, true
}


// processData processes the Cloudflare API response into TrafficData
func (tm *TrafficMonitor) processData(values []float64, timestamps []string) (*TrafficData, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("no data received from API")
	}

	// Calculate baseline (average of first half of data)
	if tm.baseline == 100.0 && len(values) > 12 {
		sum := 0.0
		for i := 0; i < len(values)/2; i++ {
			sum += values[i]
		}
		tm.baseline = float64(sum) / float64(len(values)/2)
	}

	// Normalize values to percentages
	trend := make([]float64, len(values))
	maxVal := 1.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}

	for i, v := range values {
		trend[i] = (v / maxVal) * 100.0
	}

	// Current level is the latest value
	currentLevel := trend[len(trend)-1]

	// Calculate change percentage
	var baselinePercent float64
	if len(trend) > 12 {
		sum := 0.0
		for i := 0; i < 12; i++ {
			sum += trend[i]
		}
		baselinePercent = sum / 12.0
	} else {
		baselinePercent = 100.0
	}

	changePercent := ((currentLevel - baselinePercent) / baselinePercent) * 100.0

	// Determine status
	status, emoji := tm.determineStatus(currentLevel, baselinePercent)

	// Parse timestamps
	timesList := make([]time.Time, 0, len(values))
	if len(timestamps) == len(values) && len(timestamps) > 0 {
		for _, ts := range timestamps {
			t, err := time.Parse(time.RFC3339, ts)
			if err == nil {
				timesList = append(timesList, t)
			}
		}
	}

	// If timestamps are missing or invalid, generate based on now and 1h interval
	if len(timesList) != len(values) {
		timesList = make([]time.Time, len(values))
		now := time.Now().UTC()
		for i := range values {
			timesList[i] = now.Add(-time.Duration(len(values)-i-1) * time.Hour)
		}
	}

	return &TrafficData{
		CurrentLevel:  currentLevel,
		Trend24h:      trend,
		Timestamps:    timesList,
		ChangePercent: changePercent,
		Status:        status,
		StatusEmoji:   emoji,
		LastUpdate:    time.Now(),
	}, nil
}

// determineStatus determines the traffic status based on current level vs baseline
func (tm *TrafficMonitor) determineStatus(current, baseline float64) (string, string) {
	ratio := current / baseline

	switch {
	case ratio > 0.7:
		return "Normal", "🟢"
	case ratio > 0.3:
		return "Degraded", "🟡"
	case ratio > 0.1:
		return "Throttled", "🟠"
	default:
		return "Shutdown", "🔴"
	}
}

// Start begins background monitoring
// Note: Initial fetch should already be done in PerformInitialCheck
func (tm *TrafficMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Skip initial fetch here - it's already done in PerformInitialCheck
	// This ensures Cloudflare data is fetched FIRST before bot starts

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("📡 Periodic Cloudflare Radar data fetch...")
			_, _ = tm.FetchFromCloudflare(ctx)
		}
	}
}

// FetchASNTrafficFromCloudflare fetches ASN-level traffic data from Cloudflare Radar API
// Returns top 10 Iranian ASNs by traffic volume
// Follows the same pattern as FetchFromCloudflare for consistency
// Tries multiple endpoint variations to find the correct one
func (tm *TrafficMonitor) FetchASNTrafficFromCloudflare(ctx context.Context, iranASNs []string) ([]*models.ASTrafficData, error) {
	// Try multiple endpoint variations (similar to Iran traffic retry logic)
	// Based on Cloudflare Radar API docs: /radar/netflows/top/ases for top ASNs
	endpointVariations := []string{
		// Try 1: Netflows top ASes (documented endpoint)
		"https://api.cloudflare.com/client/v4/radar/netflows/top/ases?location=IR&dateRange=1d&format=json",
		// Try 2: HTTP top ASes
		"https://api.cloudflare.com/client/v4/radar/http/top/ases?location=IR&dateRange=1d&format=json",
		// Try 3: Query parameter with dimension
		"https://api.cloudflare.com/client/v4/radar/http/top?dimension=asn&location=IR&dateRange=1d&format=json",
		// Try 4: Summary endpoint with dimension
		"https://api.cloudflare.com/client/v4/radar/http/summary?dimension=asn&location=IR&dateRange=1d&format=json",
		// Try 5: Summary/asn path
		"https://api.cloudflare.com/client/v4/radar/http/summary/asn?location=IR&dateRange=1d&format=json",
		// Try 6: Netflows endpoint (old variant)
		"https://api.cloudflare.com/client/v4/radar/netflows/top/asn?location=IR&dateRange=1d&format=json",
		// Try 7: Netflows summary
		"https://api.cloudflare.com/client/v4/radar/netflows/summary?dimension=asn&location=IR&dateRange=1d&format=json",
		// Try 8: Original (if API is fixed later)
		"https://api.cloudflare.com/client/v4/radar/http/top/asn?location=IR&dateRange=1d&format=json",
	}

	// Try each endpoint variation
	for i, url := range endpointVariations {
		log.Printf("Trying ASN endpoint variation %d/%d: %s", i+1, len(endpointVariations), url)
		result, err := tm.fetchASNTrafficWithURL(ctx, url, iranASNs)
		if err == nil && len(result) > 0 {
			log.Printf("✅ Successfully fetched ASN traffic data using endpoint variation %d", i+1)
			return result, nil
		}
		if err != nil {
			log.Printf("⚠️  Endpoint variation %d failed: %v", i+1, err)
		}
	}

	// All endpoints failed
	log.Printf("❌ All ASN endpoint variations failed - ASN traffic chart will be skipped")
	return []*models.ASTrafficData{}, nil
}

// fetchASNTrafficWithURL fetches ASN traffic data using a specific URL
// Helper function similar to fetchWithURL for Iran traffic
func (tm *TrafficMonitor) fetchASNTrafficWithURL(ctx context.Context, url string, iranASNs []string) ([]*models.ASTrafficData, error) {

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.Header.Set("User-Agent", "NetBlocks-Monitor/1.0")
	
	// Add Cloudflare authentication headers - match working pattern exactly
	if tm.cloudflareToken != "" {
		req.Header.Set("Authorization", "Bearer "+tm.cloudflareToken)
	} else if tm.cloudflareEmail != "" && tm.cloudflareKey != "" {
		req.Header.Set("X-Auth-Email", tm.cloudflareEmail)
		req.Header.Set("X-Auth-Key", tm.cloudflareKey)
	} else {
		return nil, fmt.Errorf("no Cloudflare credentials available")
	}

	resp, err := tm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		// Log error details but don't fail immediately (let other endpoints be tried)
		var errorResp struct {
			Success bool `json:"success"`
			Errors  []struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &errorResp); jsonErr == nil && len(errorResp.Errors) > 0 {
			for _, err := range errorResp.Errors {
				log.Printf("  Endpoint error %d: %s", err.Code, err.Message)
			}
		}
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	var apiResp CloudflareRadarResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		log.Printf("Error decoding ASN traffic JSON response: %v", err)
		log.Printf("Response body (first 500 chars): %s", string(bodyBytes[:min(500, len(bodyBytes))]))
		return nil, fmt.Errorf("error decoding JSON response: %w", err)
	}

	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			log.Printf("Cloudflare ASN API returned success=false with errors:")
			for _, err := range apiResp.Errors {
				log.Printf("  Error %d: %s", err.Code, err.Message)
			}
		} else {
			log.Printf("Cloudflare ASN API returned success=false (no error details provided)")
		}
		return nil, fmt.Errorf("cloudflare ASN API returned success=false")
	}

	// Parse the result to extract ASN traffic data
	// Define a structure to hold parsed ASN items
	// Note: Cloudflare API returns clientASN/clientASName for top/ases endpoints
	type asnItem struct {
		ASN         interface{} `json:"asn"`          // Standard field
		ClientASN   interface{} `json:"clientASN"`    // Used by /top/ases endpoints
		ClientASName string     `json:"clientASName"` // Used by /top/ases endpoints
		Value       interface{} `json:"value"`        // Can be string or float64
		Change      float64     `json:"change,omitempty"`
	}
	
	var summaryData []asnItem
	
	// Try structure with top_0 field first (used by /top/ases endpoints)
	var resultTop0 struct {
		Top0 []asnItem `json:"top_0"`
		Meta struct {
			DateRange []struct {
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
			} `json:"dateRange"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(apiResp.Result, &resultTop0); err == nil && len(resultTop0.Top0) > 0 {
		log.Printf("Using 'top_0' field - found %d ASN items", len(resultTop0.Top0))
		summaryData = resultTop0.Top0
	} else {
		// Try standard structure with summary/top fields
		var result struct {
			Meta struct {
				DateRange []struct {
					StartTime string `json:"startTime"`
					EndTime   string `json:"endTime"`
				} `json:"dateRange"`
			} `json:"meta"`
			Summary []asnItem `json:"summary"`
			Top     []asnItem `json:"top"`
		}

		if err := json.Unmarshal(apiResp.Result, &result); err == nil {
			// Use Summary or Top field, whichever has data
			if len(result.Summary) > 0 {
				summaryData = result.Summary
			} else if len(result.Top) > 0 {
				log.Printf("Using 'top' field instead of 'summary' - found %d items", len(result.Top))
				summaryData = result.Top
			}
		}
	}
	
	// If still no data, try to parse as raw map to see structure
	if len(summaryData) == 0 {
		log.Printf("⚠️  Could not parse ASN traffic result with expected structures")
		if len(apiResp.Result) > 0 {
			resultStr := string(apiResp.Result)
			if len(resultStr) > 1000 {
				resultStr = resultStr[:1000] + "..."
			}
			log.Printf("Response result: %s", resultStr)
		}
		
		// Try to parse as raw map to see structure
		var rawResult map[string]interface{}
		if jsonErr := json.Unmarshal(apiResp.Result, &rawResult); jsonErr == nil {
			log.Printf("Response top-level keys: %v", getKeys(rawResult))
			// Check for various possible field names
			for _, key := range []string{"summary", "top", "data", "results", "asns", "asn"} {
				if val, ok := rawResult[key]; ok {
					log.Printf("Found field '%s': %T", key, val)
					if arr, ok := val.([]interface{}); ok {
						log.Printf("  Array length: %d", len(arr))
						if len(arr) > 0 {
							log.Printf("  First item type: %T, value: %v", arr[0], arr[0])
							// Try to extract ASN data from this array
							for _, item := range arr {
								if itemMap, ok := item.(map[string]interface{}); ok {
									var asnVal interface{}
									var value float64
									// Check various possible ASN field names
									for _, asnKey := range []string{"asn", "as", "as_number", "asNumber"} {
										if asn, ok := itemMap[asnKey]; ok {
											asnVal = asn
											break
										}
									}
									// Check various possible value field names
									for _, valKey := range []string{"value", "count", "requests", "bytes", "traffic"} {
										if val, ok := itemMap[valKey].(float64); ok {
											value = val
											break
										}
									}
									if asnVal != nil && value > 0 {
										summaryData = append(summaryData, asnItem{ASN: asnVal, Value: value})
									}
								}
							}
						}
					} else if valMap, ok := val.(map[string]interface{}); ok {
						log.Printf("  Map keys: %v", getKeys(valMap))
					}
				}
			}
		}
	}

	if len(summaryData) == 0 {
		log.Printf("⚠️  No ASN traffic data available after parsing - will skip ASN chart")
		log.Printf("Full response body (first 2000 chars): %s", string(bodyBytes[:min(2000, len(bodyBytes))]))
		return []*models.ASTrafficData{}, nil
	}

	log.Printf("Cloudflare ASN API success - received %d ASNs in response", len(summaryData))

	// Calculate total traffic for percentage calculation
	// Note: values from /top/ases endpoints are already percentages, but we'll sum them for relative comparison
	var totalTraffic float64
	for _, item := range summaryData {
		var value float64
		switch v := item.Value.(type) {
		case float64:
			value = v
		case string:
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				value = parsed
			}
		case int:
			value = float64(v)
		}
		totalTraffic += value
	}

	log.Printf("Total ASN traffic from API: %f, Found %d ASNs in response", totalTraffic, len(summaryData))

	// Create a map of Iranian ASNs for quick lookup
	iranASNMap := make(map[string]bool)
	for _, asn := range iranASNs {
		// Remove "AS" prefix if present for comparison
		asnNum := strings.TrimPrefix(asn, "AS")
		iranASNMap[asnNum] = true
	}
	log.Printf("Looking for %d configured Iranian ASNs in API response", len(iranASNMap))
	
	// Log first few ASNs from API for debugging
	log.Printf("First 5 ASNs from API response:")
	for i, item := range summaryData {
		if i >= 5 {
			break
		}
		asnValue := item.ASN
		if item.ClientASN != nil {
			asnValue = item.ClientASN
		}
		var valueStr string
		switch v := item.Value.(type) {
		case float64:
			valueStr = fmt.Sprintf("%f", v)
		case string:
			valueStr = v
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		log.Printf("  ASN %v (Name: %s), Value: %s", asnValue, item.ClientASName, valueStr)
	}

	// Filter and process ASN traffic data
	asnTrafficList := make([]*models.ASTrafficData, 0)
	for _, item := range summaryData {
		// Handle ASN - can be in ASN or ClientASN field
		var asnNum int
		var asnStr, asnNumStr string
		var asnValue interface{}
		
		// Prefer ClientASN if available (from /top/ases endpoints)
		if item.ClientASN != nil {
			asnValue = item.ClientASN
		} else if item.ASN != nil {
			asnValue = item.ASN
		} else {
			log.Printf("ASN item missing both ASN and ClientASN fields - skipping")
			continue
		}
		
		// Parse ASN value
		switch v := asnValue.(type) {
		case float64:
			asnNum = int(v)
			asnStr = fmt.Sprintf("AS%d", asnNum)
			asnNumStr = fmt.Sprintf("%d", asnNum)
		case int:
			asnNum = v
			asnStr = fmt.Sprintf("AS%d", asnNum)
			asnNumStr = fmt.Sprintf("%d", asnNum)
		case string:
			asnStr = v
			asnNumStr = strings.TrimPrefix(v, "AS")
			// Try to parse as int for comparison
			if parsed, err := strconv.Atoi(asnNumStr); err == nil {
				asnNum = parsed
			}
		default:
			log.Printf("Unexpected ASN type: %T, value: %v", asnValue, asnValue)
			continue
		}
		
		// Check if this ASN is in our Iranian ASN list
		if !iranASNMap[asnNumStr] {
			continue
		}

		// Parse value - can be string or float64
		var value float64
		switch v := item.Value.(type) {
		case float64:
			value = v
		case string:
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				value = parsed
			} else {
				log.Printf("Could not parse value as float: %v", v)
				continue
			}
		case int:
			value = float64(v)
		default:
			log.Printf("Unexpected value type: %T, value: %v", item.Value, item.Value)
			continue
		}

		percentage := 0.0
		if totalTraffic > 0 {
			percentage = (value / totalTraffic) * 100.0
		}

		// Get ASN name - prefer ClientASName if available, otherwise use config
		asnName := item.ClientASName
		if asnName == "" {
			asnName = config.GetASNName(asnStr)
			if asnName == "Unknown" {
				asnName = asnStr
			}
		}

		// Determine status based on percentage
		status, emoji := tm.determineASNStatus(percentage)

		asnTrafficList = append(asnTrafficList, &models.ASTrafficData{
			ASN:          asnStr,
			Name:         asnName,
			TrafficVolume: value,
			Percentage:    percentage,
			Status:       status,
			StatusEmoji:  emoji,
			LastUpdate:   time.Now(),
		})
	}

	// Sort by traffic volume (highest first) and take top 10
	if len(asnTrafficList) > 1 {
		for i := 0; i < len(asnTrafficList)-1; i++ {
			for j := i + 1; j < len(asnTrafficList); j++ {
				if asnTrafficList[i].TrafficVolume < asnTrafficList[j].TrafficVolume {
					asnTrafficList[i], asnTrafficList[j] = asnTrafficList[j], asnTrafficList[i]
				}
			}
		}
	}

	// Limit to top 10
	if len(asnTrafficList) > 10 {
		asnTrafficList = asnTrafficList[:10]
	}

	if len(asnTrafficList) == 0 {
		log.Printf("⚠️  No Iranian ASNs matched in API response - will skip ASN chart")
		log.Printf("Configured ASN count: %d, API response ASN count: %d", len(iranASNMap), len(summaryData))
		return []*models.ASTrafficData{}, nil
	}

	// Log top ASNs - matching working chart pattern
	topNames := make([]string, 0, min(3, len(asnTrafficList)))
	for i := 0; i < min(3, len(asnTrafficList)); i++ {
		topNames = append(topNames, asnTrafficList[i].Name)
	}
	log.Printf("ASN traffic data processed successfully - %d Iranian ASNs found (top ASNs: %v)", 
		len(asnTrafficList), topNames)
	return asnTrafficList, nil
}

// determineASNStatus determines the ASN traffic status based on percentage
func (tm *TrafficMonitor) determineASNStatus(percentage float64) (string, string) {
	switch {
	case percentage >= 5.0:
		return "High", "🟢"
	case percentage >= 1.0:
		return "Medium", "🟡"
	case percentage >= 0.1:
		return "Low", "🟠"
	default:
		return "Very Low", "⚪"
	}
}

