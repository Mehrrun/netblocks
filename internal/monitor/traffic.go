package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
)

// TrafficMonitor monitors Iran's internet traffic using Cloudflare Radar API
type TrafficMonitor struct {
	client          *http.Client
	lastUpdate      time.Time
	cachedData      *TrafficData
	cached7d        *TrafficData
	last7dUpdate    time.Time
	mu              sync.RWMutex
	baseline        float64
	cloudflareToken string
	cloudflareEmail string
	cloudflareKey   string
}

// TrafficData represents Iran's internet traffic statistics (absolute volume)
type TrafficData struct {
	CurrentLevel  float64
	Baseline      float64
	Trend24h      []float64
	Trend7d       []float64
	Timestamps    []time.Time
	Timestamps7d  []time.Time
	ChangePercent float64
	Status        string
	StatusEmoji   string
	Unit          string
	Source        string
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
func NewTrafficMonitor(cloudflareToken, cloudflareEmail, cloudflareKey string) *TrafficMonitor {
	log.Printf("NewTrafficMonitor: token set=%v (len=%d), email set=%v, key set=%v",
		cloudflareToken != "", len(cloudflareToken),
		cloudflareEmail != "", cloudflareKey != "")

	return &TrafficMonitor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseline:        0,
		cloudflareToken: cloudflareToken,
		cloudflareEmail: cloudflareEmail,
		cloudflareKey:   cloudflareKey,
	}
}

// GetTrafficData returns cached or fresh 24h traffic data
func (tm *TrafficMonitor) GetTrafficData(ctx context.Context) (*TrafficData, error) {
	tm.mu.RLock()
	if tm.cachedData != nil && time.Since(tm.lastUpdate) < 5*time.Minute {
		data := tm.cachedData
		tm.mu.RUnlock()
		return data, nil
	}
	tm.mu.RUnlock()
	return tm.FetchFromCloudflare(ctx)
}

// GetTrafficData7d returns cached or fresh 7d traffic data
func (tm *TrafficMonitor) GetTrafficData7d(ctx context.Context) (*TrafficData, error) {
	tm.mu.RLock()
	if tm.cached7d != nil && time.Since(tm.last7dUpdate) < 15*time.Minute {
		data := tm.cached7d
		tm.mu.RUnlock()
		return data, nil
	}
	tm.mu.RUnlock()
	return tm.Fetch7dFromCloudflare(ctx)
}

func (tm *TrafficMonitor) setAuth(req *http.Request) string {
	if tm.cloudflareToken != "" {
		req.Header.Set("Authorization", "Bearer "+tm.cloudflareToken)
		return "Bearer Token"
	}
	if tm.cloudflareEmail != "" && tm.cloudflareKey != "" {
		req.Header.Set("X-Auth-Email", tm.cloudflareEmail)
		req.Header.Set("X-Auth-Key", tm.cloudflareKey)
		return "API Key"
	}
	return "none"
}

func (tm *TrafficMonitor) doRadarGET(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NetBlocks-Monitor/1.0")
	authMethod := tm.setAuth(req)
	if authMethod == "none" {
		log.Printf("WARNING: No Cloudflare credentials available - request will likely fail")
	}

	resp, err := tm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed (%s): %w", authMethod, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("Cloudflare API response: Status %d %s (auth: %s) url=%s",
		resp.StatusCode, resp.Status, authMethod, url)

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Errors []struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		if json.Unmarshal(bodyBytes, &errorResp) == nil {
			for _, e := range errorResp.Errors {
				log.Printf("Cloudflare API error %d: %s", e.Code, e.Message)
			}
		}
		return nil, fmt.Errorf("cloudflare API status %d", resp.StatusCode)
	}
	return bodyBytes, nil
}

// FetchFromCloudflare fetches absolute Iran NetFlows (24h) from Cloudflare Radar
func (tm *TrafficMonitor) FetchFromCloudflare(ctx context.Context) (*TrafficData, error) {
	endpoints := []struct {
		url    string
		source string
		unit   string
	}{
		{
			url:    "https://api.cloudflare.com/client/v4/radar/netflows/timeseries?location=IR&dateRange=1d&aggInterval=1h&format=json",
			source: "netflows",
			unit:   "bytes",
		},
		{
			url:    "https://api.cloudflare.com/client/v4/radar/netflows/timeseries?location=IRN&dateRange=1d&aggInterval=1h&format=json",
			source: "netflows",
			unit:   "bytes",
		},
		{
			url:    "https://api.cloudflare.com/client/v4/radar/http/timeseries?location=IR&dateRange=1d&aggInterval=1h&format=json",
			source: "http",
			unit:   "requests",
		},
		{
			url:    "https://api.cloudflare.com/client/v4/radar/http/timeseries?location=IRN&dateRange=1d&aggInterval=1h&format=json",
			source: "http",
			unit:   "requests",
		},
	}

	var lastErr error
	for _, ep := range endpoints {
		log.Printf("Fetching Cloudflare Radar data from: %s", ep.url)
		body, err := tm.doRadarGET(ctx, ep.url)
		if err != nil {
			lastErr = err
			continue
		}

		var apiResp CloudflareRadarResponse
		if err := json.Unmarshal(body, &apiResp); err != nil || !apiResp.Success {
			lastErr = fmt.Errorf("invalid radar response")
			continue
		}

		timestamps, values, unit, found := extractSeriesWithMeta(apiResp.Result)
		if !found || len(values) == 0 {
			lastErr = fmt.Errorf("no traffic data in response")
			continue
		}
		if unit == "" {
			unit = ep.unit
		}

		data, err := tm.processData(values, timestamps, unit, ep.source)
		if err != nil {
			lastErr = err
			continue
		}

		log.Printf("Traffic data processed - Current: %s, Status: %s %s (source=%s)",
			formatAbsoluteValue(data.CurrentLevel, data.Unit), data.StatusEmoji, data.Status, data.Source)

		tm.mu.Lock()
		tm.cachedData = data
		tm.lastUpdate = time.Now()
		tm.mu.Unlock()
		return data, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no traffic data available")
	}
	return nil, lastErr
}

// Fetch7dFromCloudflare fetches absolute Iran NetFlows for 7 days
func (tm *TrafficMonitor) Fetch7dFromCloudflare(ctx context.Context) (*TrafficData, error) {
	endpoints := []struct {
		url    string
		source string
		unit   string
	}{
		{
			url:    "https://api.cloudflare.com/client/v4/radar/netflows/timeseries?location=IR&dateRange=7d&aggInterval=1h&format=json",
			source: "netflows",
			unit:   "bytes",
		},
		{
			url:    "https://api.cloudflare.com/client/v4/radar/netflows/timeseries?location=IRN&dateRange=7d&aggInterval=1h&format=json",
			source: "netflows",
			unit:   "bytes",
		},
		{
			url:    "https://api.cloudflare.com/client/v4/radar/http/timeseries?location=IR&dateRange=7d&aggInterval=1h&format=json",
			source: "http",
			unit:   "requests",
		},
	}

	var lastErr error
	for _, ep := range endpoints {
		log.Printf("Fetching Cloudflare Radar 7d data from: %s", ep.url)
		body, err := tm.doRadarGET(ctx, ep.url)
		if err != nil {
			lastErr = err
			continue
		}

		var apiResp CloudflareRadarResponse
		if err := json.Unmarshal(body, &apiResp); err != nil || !apiResp.Success {
			lastErr = fmt.Errorf("invalid radar response")
			continue
		}

		timestamps, values, unit, found := extractSeriesWithMeta(apiResp.Result)
		if !found || len(values) == 0 {
			lastErr = fmt.Errorf("no 7d traffic data")
			continue
		}
		if unit == "" {
			unit = ep.unit
		}

		data, err := tm.processData7d(values, timestamps, unit, ep.source)
		if err != nil {
			lastErr = err
			continue
		}

		tm.mu.Lock()
		tm.cached7d = data
		tm.last7dUpdate = time.Now()
		tm.mu.Unlock()
		return data, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no 7d traffic data available")
	}
	return nil, lastErr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

type radarSerie struct {
	Timestamps []string      `json:"timestamps"`
	Values     []interface{} `json:"values"`
}

func extractSeriesWithMeta(resultRaw json.RawMessage) ([]string, []float64, string, bool) {
	unit := ""
	var metaWrap struct {
		Meta struct {
			Units []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"units"`
			Normalization string `json:"normalization"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(resultRaw, &metaWrap)
	if len(metaWrap.Meta.Units) > 0 && metaWrap.Meta.Units[0].Value != "" {
		unit = metaWrap.Meta.Units[0].Value
	}

	ts, vals, ok := extractSeries(resultRaw)
	return ts, vals, unit, ok
}

func extractSeries(resultRaw json.RawMessage) ([]string, []float64, bool) {
	var rr struct {
		Serie0     *radarSerie  `json:"serie_0"`
		Serie0Alt  *radarSerie  `json:"serie0"`
		Series     []radarSerie `json:"series"`
		Data       *radarSerie  `json:"data"`
		Timeseries []radarSerie `json:"timeseries"`
		Timestamps []string     `json:"timestamps"`
		Values     []interface{} `json:"values"`
	}
	if err := json.Unmarshal(resultRaw, &rr); err == nil {
		if vals := toFloatSlice(rr.Values); len(vals) > 0 && len(rr.Timestamps) > 0 {
			return rr.Timestamps, vals, true
		}
		if rr.Serie0 != nil {
			if vals := toFloatSlice(rr.Serie0.Values); len(vals) > 0 {
				return rr.Serie0.Timestamps, vals, true
			}
		}
		if rr.Serie0Alt != nil {
			if vals := toFloatSlice(rr.Serie0Alt.Values); len(vals) > 0 {
				return rr.Serie0Alt.Timestamps, vals, true
			}
		}
		if len(rr.Series) > 0 {
			if vals := toFloatSlice(rr.Series[0].Values); len(vals) > 0 {
				return rr.Series[0].Timestamps, vals, true
			}
		}
		if rr.Data != nil {
			if vals := toFloatSlice(rr.Data.Values); len(vals) > 0 {
				return rr.Data.Timestamps, vals, true
			}
		}
		if len(rr.Timeseries) > 0 {
			if vals := toFloatSlice(rr.Timeseries[0].Values); len(vals) > 0 {
				return rr.Timeseries[0].Timestamps, vals, true
			}
		}
	}

	var raw map[string]interface{}
	if json.Unmarshal(resultRaw, &raw) != nil {
		return nil, nil, false
	}
	for _, key := range []string{"serie_0", "serie0", "series", "data", "timeseries"} {
		if v, ok := raw[key]; ok {
			if ts, vals, ok := parseSerie(v); ok {
				return ts, vals, true
			}
		}
	}
	if ts, vals, ok := parseSerie(raw); ok {
		return ts, vals, true
	}
	return nil, nil, false
}

func parseSerie(v interface{}) ([]string, []float64, bool) {
	switch s := v.(type) {
	case map[string]interface{}:
		timestamps := toStringSlice(s["timestamps"])
		values := toFloatSlice(s["values"])
		if len(values) == 0 {
			values = toFloatSlice(s["value"])
		}
		if len(values) > 0 && len(timestamps) > 0 {
			return timestamps, values, true
		}
		if len(values) == 0 {
			if ts, vals, ok := parseSeriesPairs(s["data"]); ok {
				return ts, vals, true
			}
		}
		for _, item := range s {
			if ts, vals, ok := parseSerie(item); ok {
				return ts, vals, true
			}
		}
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

func parseTimestamps(timestamps []string, n int, step time.Duration) []time.Time {
	timesList := make([]time.Time, 0, n)
	if len(timestamps) == n && n > 0 {
		for _, ts := range timestamps {
			t, err := time.Parse(time.RFC3339, ts)
			if err == nil {
				timesList = append(timesList, t)
			}
		}
	}
	if len(timesList) != n {
		timesList = make([]time.Time, n)
		now := time.Now().UTC()
		for i := range timesList {
			timesList[i] = now.Add(-time.Duration(n-i-1) * step)
		}
	}
	return timesList
}

// processData keeps absolute values (no peak normalization)
func (tm *TrafficMonitor) processData(values []float64, timestamps []string, unit, source string) (*TrafficData, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("no data received from API")
	}

	trend := make([]float64, len(values))
	copy(trend, values)

	currentLevel := trend[len(trend)-1]

	baselineCount := len(trend) / 2
	if baselineCount < 1 {
		baselineCount = 1
	}
	if baselineCount > 12 {
		baselineCount = 12
	}
	sum := 0.0
	for i := 0; i < baselineCount; i++ {
		sum += trend[i]
	}
	baseline := sum / float64(baselineCount)
	if baseline <= 0 {
		baseline = currentLevel
		if baseline <= 0 {
			baseline = 1
		}
	}
	tm.baseline = baseline

	changePercent := ((currentLevel - baseline) / baseline) * 100.0
	status, emoji := tm.determineStatus(currentLevel, baseline)

	return &TrafficData{
		CurrentLevel:  currentLevel,
		Baseline:      baseline,
		Trend24h:      trend,
		Timestamps:    parseTimestamps(timestamps, len(values), time.Hour),
		ChangePercent: changePercent,
		Status:        status,
		StatusEmoji:   emoji,
		Unit:          unit,
		Source:        source,
		LastUpdate:    time.Now(),
	}, nil
}

func (tm *TrafficMonitor) processData7d(values []float64, timestamps []string, unit, source string) (*TrafficData, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("no 7d data received from API")
	}
	trend := make([]float64, len(values))
	copy(trend, values)
	current := trend[len(trend)-1]
	baselineCount := len(trend) / 2
	if baselineCount < 1 {
		baselineCount = 1
	}
	sum := 0.0
	for i := 0; i < baselineCount; i++ {
		sum += trend[i]
	}
	baseline := sum / float64(baselineCount)
	if baseline <= 0 {
		baseline = current
		if baseline <= 0 {
			baseline = 1
		}
	}
	changePercent := ((current - baseline) / baseline) * 100.0
	status, emoji := tm.determineStatus(current, baseline)

	return &TrafficData{
		CurrentLevel:  current,
		Baseline:      baseline,
		Trend7d:       trend,
		Timestamps7d:  parseTimestamps(timestamps, len(values), time.Hour),
		ChangePercent: changePercent,
		Status:        status,
		StatusEmoji:   emoji,
		Unit:          unit,
		Source:        source,
		LastUpdate:    time.Now(),
	}, nil
}

func (tm *TrafficMonitor) determineStatus(current, baseline float64) (string, string) {
	if baseline <= 0 {
		return "Normal", "🟢"
	}
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("📡 Periodic Cloudflare Radar data fetch...")
			_, _ = tm.FetchFromCloudflare(ctx)
			_, _ = tm.Fetch7dFromCloudflare(ctx)
		}
	}
}

// FetchASNTrafficFromCloudflare fetches top Iranian ASNs by NetFlows share
func (tm *TrafficMonitor) FetchASNTrafficFromCloudflare(ctx context.Context) ([]*models.ASTrafficData, error) {
	endpointVariations := []string{
		"https://api.cloudflare.com/client/v4/radar/netflows/top/ases?location=IR&dateRange=1d&limit=20&format=json",
		"https://api.cloudflare.com/client/v4/radar/netflows/top/ases?location=IRN&dateRange=1d&limit=20&format=json",
		"https://api.cloudflare.com/client/v4/radar/http/top/ases?location=IR&dateRange=1d&limit=20&format=json",
	}

	for i, url := range endpointVariations {
		log.Printf("Trying ASN endpoint %d/%d: %s", i+1, len(endpointVariations), url)
		result, err := tm.fetchASNTrafficWithURL(ctx, url)
		if err == nil && len(result) > 0 {
			log.Printf("✅ ASN traffic data from endpoint %d (%d ASNs)", i+1, len(result))
			return result, nil
		}
		if err != nil {
			log.Printf("⚠️  ASN endpoint %d failed: %v", i+1, err)
		}
	}

	log.Printf("❌ All ASN endpoint variations failed - ASN traffic chart will be skipped")
	return []*models.ASTrafficData{}, nil
}

func (tm *TrafficMonitor) fetchASNTrafficWithURL(ctx context.Context, url string) ([]*models.ASTrafficData, error) {
	bodyBytes, err := tm.doRadarGET(ctx, url)
	if err != nil {
		return nil, err
	}

	var apiResp CloudflareRadarResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("error decoding JSON response: %w", err)
	}
	if !apiResp.Success {
		return nil, fmt.Errorf("cloudflare ASN API returned success=false")
	}

	type asnItem struct {
		ASN          interface{} `json:"asn"`
		ClientASN    interface{} `json:"clientASN"`
		ClientASName string      `json:"clientASName"`
		Value        interface{} `json:"value"`
	}

	var summaryData []asnItem
	var resultTop0 struct {
		Top0 []asnItem `json:"top_0"`
	}
	if err := json.Unmarshal(apiResp.Result, &resultTop0); err == nil && len(resultTop0.Top0) > 0 {
		summaryData = resultTop0.Top0
	} else {
		var result struct {
			Summary []asnItem `json:"summary"`
			Top     []asnItem `json:"top"`
		}
		if err := json.Unmarshal(apiResp.Result, &result); err == nil {
			if len(result.Summary) > 0 {
				summaryData = result.Summary
			} else if len(result.Top) > 0 {
				summaryData = result.Top
			}
		}
	}

	if len(summaryData) == 0 {
		return []*models.ASTrafficData{}, nil
	}

	var totalTraffic float64
	parsed := make([]struct {
		asnStr string
		name   string
		value  float64
	}, 0, len(summaryData))

	for _, item := range summaryData {
		asnValue := item.ClientASN
		if asnValue == nil {
			asnValue = item.ASN
		}
		if asnValue == nil {
			continue
		}

		var asnStr string
		switch v := asnValue.(type) {
		case float64:
			asnStr = fmt.Sprintf("AS%d", int(v))
		case int:
			asnStr = fmt.Sprintf("AS%d", v)
		case string:
			asnStr = v
			if !strings.HasPrefix(strings.ToUpper(asnStr), "AS") {
				asnStr = "AS" + asnStr
			}
		default:
			continue
		}

		value, ok := toFloat(item.Value)
		if !ok {
			continue
		}

		asnName := item.ClientASName
		if asnName == "" {
			asnName = config.GetASNName(asnStr)
			if asnName == "Unknown" {
				asnName = asnStr
			}
		} else {
			// Prefer friendly config name when available
			if mapped := config.GetASNName(asnStr); mapped != "Unknown" {
				asnName = mapped
			}
		}

		totalTraffic += value
		parsed = append(parsed, struct {
			asnStr string
			name   string
			value  float64
		}{asnStr, asnName, value})
	}

	asnTrafficList := make([]*models.ASTrafficData, 0, len(parsed))
	for _, item := range parsed {
		percentage := item.value
		// If values look like raw counts (sum >> 100), convert to share of total
		if totalTraffic > 100.5 {
			percentage = (item.value / totalTraffic) * 100.0
		}
		status, emoji := tm.determineASNStatus(percentage)
		asnTrafficList = append(asnTrafficList, &models.ASTrafficData{
			ASN:           item.asnStr,
			Name:          item.name,
			TrafficVolume: item.value,
			Percentage:    percentage,
			Status:        status,
			StatusEmoji:   emoji,
			LastUpdate:    time.Now(),
		})
	}

	sort.Slice(asnTrafficList, func(i, j int) bool {
		return asnTrafficList[i].Percentage > asnTrafficList[j].Percentage
	})

	if len(asnTrafficList) > 20 {
		asnTrafficList = asnTrafficList[:20]
	}

	return asnTrafficList, nil
}

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

// formatAbsoluteValue formats a raw Radar value for logs/captions
func formatAbsoluteValue(v float64, unit string) string {
	u := strings.ToLower(unit)
	switch {
	case strings.Contains(u, "byte"):
		return formatBytes(v)
	case strings.Contains(u, "request"):
		return formatCompact(v) + " req"
	default:
		return formatCompact(v)
	}
}

func formatBytes(v float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	x := v
	i := 0
	for x >= 1024 && i < len(units)-1 {
		x /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", x, units[i])
	}
	return fmt.Sprintf("%.2f %s", x, units[i])
}

func formatCompact(v float64) string {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1e12:
		return fmt.Sprintf("%.2fT", v/1e12)
	case abs >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("%.2fK", v/1e3)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}
