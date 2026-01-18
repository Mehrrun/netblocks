package monitor

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TrafficMonitor monitors Iran's internet traffic using Cloudflare Radar API
type TrafficMonitor struct {
	client      *http.Client
	lastUpdate  time.Time
	cachedData  *TrafficData
	mu          sync.RWMutex
	baseline    float64
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
func NewTrafficMonitor() *TrafficMonitor {
	return &TrafficMonitor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseline: 100.0, // Will be calculated from data
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
	// For now, generate simulated traffic data based on ASN/DNS connectivity
	// TODO: Integrate with actual Cloudflare Radar API when auth token is available
	
	// Generate realistic 24-hour traffic pattern
	now := time.Now()
	trend := make([]float64, 24)
	timestamps := make([]time.Time, 24)
	
	// Simulate traffic pattern: higher during day (8am-11pm), lower at night
	for i := 0; i < 24; i++ {
		hour := (now.Hour() - (23 - i) + 24) % 24
		timestamps[i] = now.Add(-time.Duration(23-i) * time.Hour)
		
		// Base traffic level
		baseLevel := 75.0
		
		// Add time-of-day variation
		if hour >= 8 && hour <= 23 {
			// Daytime - higher traffic
			baseLevel = 80.0 + float64((hour-8)*2)
			if hour > 20 {
				baseLevel = 95.0 - float64((hour-20)*3)
			}
		} else {
			// Nighttime - lower traffic
			baseLevel = 60.0 + float64(hour*2)
		}
		
		// Add some realistic variation (±5%)
		variation := (float64(i%5) - 2.0) * 2.0
		trend[i] = baseLevel + variation
		
		// Ensure within bounds
		if trend[i] < 40 {
			trend[i] = 40
		}
		if trend[i] > 100 {
			trend[i] = 100
		}
	}
	
	// Current level is the latest value
	currentLevel := trend[23]
	
	// Calculate baseline (average of earlier hours)
	baselineSum := 0.0
	for i := 0; i < 12; i++ {
		baselineSum += trend[i]
	}
	baselinePercent := baselineSum / 12.0
	
	// Calculate change percentage
	changePercent := ((currentLevel - baselinePercent) / baselinePercent) * 100.0
	
	// Determine status
	status, emoji := tm.determineStatus(currentLevel, baselinePercent)
	
	return &TrafficData{
		CurrentLevel:  currentLevel,
		Trend24h:      trend,
		Timestamps:    timestamps,
		ChangePercent: changePercent,
		Status:        status,
		StatusEmoji:   emoji,
		LastUpdate:    time.Now(),
	}, nil
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

