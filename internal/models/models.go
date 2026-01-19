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
	Server     string    `json:"server"`
	Name       string    `json:"name"`
	Alive      bool      `json:"alive"`
	ResponseTime time.Duration `json:"response_time"`
	LastCheck  time.Time `json:"last_check"`
	Error      string    `json:"error,omitempty"`
}

// MonitoringConfig holds the configuration for monitoring
type MonitoringConfig struct {
	Interval      time.Duration `json:"interval"`
	RISLiveURL    string        `json:"ris_live_url"`
	DNSServers    []string      `json:"dns_servers"`
	IranASNs      []string      `json:"iran_asns"`
}

// MonitoringResult contains the results of a monitoring check
type MonitoringResult struct {
	Timestamp    time.Time              `json:"timestamp"`
	ASNStatuses  map[string]*ASNStatus  `json:"asn_statuses"`
	DNSStatuses  map[string]*DNSStatus  `json:"dns_statuses"`
	TrafficData  *TrafficData           `json:"traffic_data,omitempty"`
	ASTrafficData []*ASTrafficData      `json:"as_traffic_data,omitempty"`
}

// ASTrafficData represents traffic statistics for a specific ASN
type ASTrafficData struct {
	ASN            string        `json:"asn"`
	Name           string        `json:"name"`
	TrafficVolume  float64       `json:"traffic_volume"`  // Bytes or requests
	Percentage     float64       `json:"percentage"`      // Percentage of total Iranian traffic
	Status         string        `json:"status"`          // Status indicator
	StatusEmoji    string        `json:"status_emoji"`
	ChartBuffer    *bytes.Buffer `json:"-"`               // PNG chart, not serialized to JSON
	LastUpdate     time.Time     `json:"last_update"`
}

// TrafficData represents Iran's internet traffic statistics
type TrafficData struct {
	CurrentLevel  float64       `json:"current_level"`
	Trend24h      []float64     `json:"trend_24h"`
	Timestamps    []time.Time   `json:"timestamps"`
	ChangePercent float64       `json:"change_percent"`
	Status        string        `json:"status"`
	StatusEmoji   string        `json:"status_emoji"`
	ChartBuffer   *bytes.Buffer `json:"-"` // PNG chart, not serialized to JSON
	LastUpdate    time.Time     `json:"last_update"`
}

