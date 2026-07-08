package models

import (
	"bytes"
	"time"
)

// ASNStatus represents the connectivity status of an Autonomous System
type ASNStatus struct {
	ASN        string    `json:"asn"`
	Country    string    `json:"country"`
	Name       string    `json:"name"`
	Connected  bool      `json:"connected"`
	LastSeen   time.Time `json:"last_seen"`
	LastUpdate time.Time `json:"last_update"`
}

// DNSStatus represents the status of a DNS server
type DNSStatus struct {
	Server       string        `json:"server"`
	Name         string        `json:"name"`
	Alive        bool          `json:"alive"`
	ResponseTime time.Duration `json:"response_time"`
	LastCheck    time.Time     `json:"last_check"`
	Error        string        `json:"error,omitempty"`
}

// MonitoringConfig holds the configuration for monitoring
type MonitoringConfig struct {
	Interval   time.Duration `json:"interval"`
	RISLiveURL string        `json:"ris_live_url"`
	DNSServers []string      `json:"dns_servers"`
	IranASNs   []string      `json:"iran_asns"`
}

// MonitoringResult contains the results of a monitoring check
type MonitoringResult struct {
	Timestamp     time.Time             `json:"timestamp"`
	ASNStatuses   map[string]*ASNStatus `json:"asn_statuses"`
	DNSStatuses   map[string]*DNSStatus `json:"dns_statuses"`
	TrafficData   *TrafficData          `json:"traffic_data,omitempty"`
	ASTrafficData []*ASTrafficData      `json:"as_traffic_data,omitempty"`
}

// ASTrafficData represents traffic statistics for a specific ASN
type ASTrafficData struct {
	ASN           string        `json:"asn"`
	Name          string        `json:"name"`
	TrafficVolume float64       `json:"traffic_volume"` // Raw share value from API
	Percentage    float64       `json:"percentage"`     // Percentage of total Iranian traffic
	Status        string        `json:"status"`
	StatusEmoji   string        `json:"status_emoji"`
	ChartBuffer   *bytes.Buffer `json:"-"`
	LastUpdate    time.Time     `json:"last_update"`
}

// TrafficData represents Iran's internet traffic statistics (absolute volume)
type TrafficData struct {
	CurrentLevel   float64       `json:"current_level"`   // Absolute latest sample
	Baseline       float64       `json:"baseline"`        // Absolute baseline for status
	Trend24h       []float64     `json:"trend_24h"`       // Absolute values (24h)
	Trend7d        []float64     `json:"trend_7d,omitempty"`
	Timestamps     []time.Time   `json:"timestamps"`
	Timestamps7d   []time.Time   `json:"timestamps_7d,omitempty"`
	ChangePercent  float64       `json:"change_percent"`
	Status         string        `json:"status"`
	StatusEmoji    string        `json:"status_emoji"`
	Unit           string        `json:"unit"` // e.g. "bytes", "requests"
	Source         string        `json:"source,omitempty"`
	ChartBuffer    *bytes.Buffer `json:"-"` // 24h PNG
	Chart7dBuffer  *bytes.Buffer `json:"-"` // 7d PNG
	LastUpdate     time.Time     `json:"last_update"`
}
