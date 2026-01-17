package monitor

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
)

// Monitor coordinates BGP and DNS monitoring
type Monitor struct {
	bgpClient  *RISLiveClient
	dnsMonitor *DNSMonitor
	config     *config.Config
	results    *models.MonitoringResult
}

// NewMonitor creates a new monitor instance
func NewMonitor(cfg *config.Config) (*Monitor, error) {
	// Initialize RIS Live client
	bgpClient, err := NewRISLiveClient(cfg.RISLiveURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create RIS Live client: %w", err)
	}

	// Subscribe to all Iranian ASNs
	for _, asn := range cfg.IranASNs {
		if err := bgpClient.SubscribeToASN(asn); err != nil {
			log.Printf("Warning: Failed to subscribe to ASN %s: %v", asn, err)
		}
	}

	bgpClient.Start()

	// Initialize DNS monitor with 8 second timeout for better reliability
	dnsMonitor := NewDNSMonitor(cfg.DNSServers, 8*time.Second)

	return &Monitor{
		bgpClient:  bgpClient,
		dnsMonitor: dnsMonitor,
		config:     cfg,
		results: &models.MonitoringResult{
			Timestamp:   time.Now(),
			ASNStatuses: make(map[string]*models.ASNStatus),
			DNSStatuses: make(map[string]*models.DNSStatus),
		},
	}, nil
}

// PerformInitialCheck performs an initial synchronous check of all monitors
// This ensures results are available before the first status display
func (m *Monitor) PerformInitialCheck(ctx context.Context) {
	// Perform initial DNS check synchronously
	_ = m.dnsMonitor.CheckAll(ctx)
	
	// Ensure BGP client has started and is ready
	// (BGP statuses are event-driven and will update as messages arrive)
	// Give a brief moment for WebSocket connection to stabilize
	time.Sleep(1 * time.Second)
	
	// Update results with initial data
	m.updateResults()
}

// Start starts monitoring
func (m *Monitor) Start(ctx context.Context) {
	// Start DNS periodic checks
	go m.dnsMonitor.StartPeriodicCheck(ctx, m.config.Interval)

	// Start periodic BGP connectivity checks
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.updateResults()
		}
	}
}

// GetResults returns current monitoring results
func (m *Monitor) GetResults() *models.MonitoringResult {
	m.updateResults()
	return m.results
}

func (m *Monitor) updateResults() {
	asnStatuses := m.bgpClient.CheckConnectivity()
	dnsStatuses := m.dnsMonitor.GetStatuses()

	m.results = &models.MonitoringResult{
		Timestamp:   time.Now(),
		ASNStatuses: asnStatuses,
		DNSStatuses: dnsStatuses,
	}
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.bgpClient != nil {
		m.bgpClient.Stop()
	}
}

