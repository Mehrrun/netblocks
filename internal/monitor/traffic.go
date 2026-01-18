package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
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
	Success bool `json:"success"`
	Result  struct {
		Serie0 struct {
			Timestamps []string `json:"timestamps"`
			Values     []int64  `json:"values"`
		} `json:"serie_0"`
	} `json:"result"`
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// NewTrafficMonitor creates a new traffic monitor
// Accepts either API Token (cloudflareToken) or API Key (cloudflareEmail + cloudflareKey)
// API Token is preferred for security
func NewTrafficMonitor(cloudflareToken, cloudflareEmail, cloudflareKey string) *TrafficMonitor {
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
// Always returns data - falls back to default if API fails
func (tm *TrafficMonitor) GetTrafficData(ctx context.Context) (*TrafficData, error) {
	tm.mu.RLock()
	// Return cached data if fresh (less than 5 minutes old)
	if tm.cachedData != nil && time.Since(tm.lastUpdate) < 5*time.Minute {
		data := tm.cachedData
		tm.mu.RUnlock()
		return data, nil
	}
	tm.mu.RUnlock()

	// Fetch fresh data (will return default if API fails)
	return tm.FetchFromCloudflare(ctx)
}

// FetchFromCloudflare fetches traffic data from Cloudflare Radar API
// Falls back to default values (1% connection) if API fails
func (tm *TrafficMonitor) FetchFromCloudflare(ctx context.Context) (*TrafficData, error) {
	// Cloudflare Radar API endpoint for Iran HTTP traffic
	url := "https://api.cloudflare.com/client/v4/radar/http/timeseries_groups/bandwidth?location=IR&dateRange=24h&aggInterval=1h"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return tm.getDefaultData(), nil
	}

	req.Header.Set("User-Agent", "NetBlocks-Monitor/1.0")
	
	// Add Cloudflare authentication headers
	if tm.cloudflareToken != "" {
		req.Header.Set("Authorization", "Bearer "+tm.cloudflareToken)
	} else if tm.cloudflareEmail != "" && tm.cloudflareKey != "" {
		req.Header.Set("X-Auth-Email", tm.cloudflareEmail)
		req.Header.Set("X-Auth-Key", tm.cloudflareKey)
	}

	resp, err := tm.client.Do(req)
	if err != nil {
		return tm.getDefaultData(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return tm.getDefaultData(), nil
	}

	var apiResp CloudflareRadarResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return tm.getDefaultData(), nil
	}

	if !apiResp.Success || len(apiResp.Result.Serie0.Values) == 0 {
		return tm.getDefaultData(), nil
	}

	// Process the data
	data, err := tm.processData(&apiResp)
	if err != nil {
		return tm.getDefaultData(), nil
	}

	// Cache the data
	tm.mu.Lock()
	tm.cachedData = data
	tm.lastUpdate = time.Now()
	tm.mu.Unlock()

	return data, nil
}

// getDefaultData returns default traffic data (1% connection) when API fails
func (tm *TrafficMonitor) getDefaultData() *TrafficData {
	// Generate 24 hours of data points with 1% connection
	hours := 24
	trend := make([]float64, hours)
	timestamps := make([]time.Time, hours)
	
	now := time.Now()
	for i := 0; i < hours; i++ {
		trend[i] = 1.0 // 1% connection
		timestamps[i] = now.Add(-time.Duration(hours-i-1) * time.Hour)
	}

	return &TrafficData{
		CurrentLevel:  1.0,
		Trend24h:     trend,
		Timestamps:   timestamps,
		ChangePercent: 0.0,
		Status:       "Shutdown",
		StatusEmoji:  "🔴",
		LastUpdate:   time.Now(),
	}
}

// processData processes the Cloudflare API response into TrafficData
func (tm *TrafficMonitor) processData(resp *CloudflareRadarResponse) (*TrafficData, error) {
	if len(resp.Result.Serie0.Values) == 0 {
		return nil, fmt.Errorf("no data received from API")
	}

	values := resp.Result.Serie0.Values
	timestamps := resp.Result.Serie0.Timestamps

	// Calculate baseline (average of first half of data)
	if tm.baseline == 100.0 && len(values) > 12 {
		sum := int64(0)
		for i := 0; i < len(values)/2; i++ {
			sum += values[i]
		}
		tm.baseline = float64(sum) / float64(len(values)/2)
	}

	// Normalize values to percentages
	trend := make([]float64, len(values))
	maxVal := int64(1)
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}

	for i, v := range values {
		trend[i] = (float64(v) / float64(maxVal)) * 100.0
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
	timesList := make([]time.Time, len(timestamps))
	for i, ts := range timestamps {
		t, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			timesList[i] = t
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
func (tm *TrafficMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Initial fetch
	_, _ = tm.FetchFromCloudflare(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = tm.FetchFromCloudflare(ctx)
		}
	}
}

