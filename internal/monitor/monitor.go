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
	bgpClient      *RISLiveClient
	dnsMonitor     *DNSMonitor
	trafficMonitor *TrafficMonitor
	config         *config.Config
	results        *models.MonitoringResult
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

	// Initialize Traffic monitor with Cloudflare credentials
	// Supports both API Token (preferred) and API Key (legacy)
	trafficMonitor := NewTrafficMonitor(cfg.CloudflareToken, cfg.CloudflareEmail, cfg.CloudflareKey)

	return &Monitor{
		bgpClient:      bgpClient,
		dnsMonitor:     dnsMonitor,
		trafficMonitor: trafficMonitor,
		config:         cfg,
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
	
	// Fetch initial traffic data
	_, _ = m.trafficMonitor.FetchFromCloudflare(ctx)
	
	// Ensure BGP client has started and is ready
	// (BGP statuses are event-driven and will update as messages arrive)
	// Give a brief moment for WebSocket connection to stabilize
	time.Sleep(1 * time.Second)
	
	// Update results with initial data
	m.updateResults(ctx)
}

// Start starts monitoring
func (m *Monitor) Start(ctx context.Context) {
	// Start DNS periodic checks
	go m.dnsMonitor.StartPeriodicCheck(ctx, m.config.Interval)

	// Start traffic monitoring in background
	go m.trafficMonitor.Start(ctx)

	// Start periodic BGP connectivity checks
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.updateResults(ctx)
		}
	}
}

// GetResults returns current monitoring results
func (m *Monitor) GetResults() *models.MonitoringResult {
	m.updateResults(context.Background())
	return m.results
}

func (m *Monitor) updateResults(ctx context.Context) {
	asnStatuses := m.bgpClient.CheckConnectivity()
	dnsStatuses := m.dnsMonitor.GetStatuses()
	
	// Get traffic data (will use cache if fresh)
	trafficData, err := m.trafficMonitor.GetTrafficData(ctx)
	if err != nil {
		log.Printf("⚠️  Failed to get traffic data: %v", err)
		log.Printf("   Traffic charts will not be available in this update")
		trafficData = nil
	}
	
	// Generate chart if we have traffic data
	var trafficModelData *models.TrafficData
	if trafficData != nil {
		log.Printf("📊 Processing traffic data - Current Level: %.1f%%, Status: %s %s", 
			trafficData.CurrentLevel, trafficData.StatusEmoji, trafficData.Status)
		
		// Generate chart
		log.Printf("📈 Generating traffic chart...")
		chartBuffer, err := GenerateTrafficChart(trafficData)
		if err != nil {
			log.Printf("❌ Failed to generate traffic chart: %v", err)
			log.Printf("   Chart will not be included in this update, but traffic status text will still be sent")
		} else if chartBuffer == nil || chartBuffer.Len() == 0 {
			log.Printf("❌ Generated chart buffer is empty or nil")
		} else {
			log.Printf("✅ Traffic chart generated successfully (size: %d bytes)", chartBuffer.Len())
		}
		
		// Convert to models.TrafficData (always create it, even if chart failed)
		// This ensures traffic status text is still sent
		trafficModelData = &models.TrafficData{
			CurrentLevel:  trafficData.CurrentLevel,
			Trend24h:      trafficData.Trend24h,
			Timestamps:    trafficData.Timestamps,
			ChangePercent: trafficData.ChangePercent,
			Status:        trafficData.Status,
			StatusEmoji:   trafficData.StatusEmoji,
			ChartBuffer:   chartBuffer, // Will be nil if generation failed
			LastUpdate:    trafficData.LastUpdate,
		}
	} else {
		log.Printf("⚠️  No traffic data available - skipping chart generation")
	}

	m.results = &models.MonitoringResult{
		Timestamp:   time.Now(),
		ASNStatuses: asnStatuses,
		DNSStatuses: dnsStatuses,
		TrafficData: trafficModelData,
	}
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.bgpClient != nil {
		m.bgpClient.Stop()
	}
}

