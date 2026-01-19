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
// IMPORTANT: Fetches Cloudflare data FIRST, then DNS, then BGP
func (m *Monitor) PerformInitialCheck(ctx context.Context) {
	// Fetch Cloudflare traffic data FIRST (most important - used for diagram)
	log.Println("📡 Fetching Cloudflare Radar data for Iran...")
	trafficData, err := m.trafficMonitor.FetchFromCloudflare(ctx)
	if err != nil {
		log.Printf("⚠️  Cloudflare fetch error (will use defaults): %v", err)
	} else if trafficData != nil {
		log.Printf("✅ Cloudflare data fetched successfully - Current Level: %.1f%%, Status: %s %s", 
			trafficData.CurrentLevel, trafficData.StatusEmoji, trafficData.Status)
	} else {
		log.Println("⚠️  Cloudflare data is nil (will use defaults)")
	}
	
	// Perform initial DNS check synchronously
	log.Println("🔍 Checking DNS servers...")
	_ = m.dnsMonitor.CheckAll(ctx)
	
	// Ensure BGP client has started and is ready
	// (BGP statuses are event-driven and will update as messages arrive)
	// Give a brief moment for WebSocket connection to stabilize
	time.Sleep(1 * time.Second)
	
	// Update results with initial data (Cloudflare data should be ready now)
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
	
	// Get traffic data (will use cache if fresh; nil on error)
	trafficData, _ := m.trafficMonitor.GetTrafficData(ctx)
	
	// Generate chart
	var trafficModelData *models.TrafficData
	if trafficData != nil {
		chartBuffer, err := GenerateTrafficChart(trafficData)
		if err != nil {
			chartBuffer = nil
		}
		
		trafficModelData = &models.TrafficData{
			CurrentLevel:  trafficData.CurrentLevel,
			Trend24h:      trafficData.Trend24h,
			Timestamps:    trafficData.Timestamps,
			ChangePercent: trafficData.ChangePercent,
			Status:        trafficData.Status,
			StatusEmoji:   trafficData.StatusEmoji,
			ChartBuffer:   chartBuffer,
			LastUpdate:    trafficData.LastUpdate,
		}
	}

	// Fetch ASN-level traffic data
	var asnTrafficList []*models.ASTrafficData
	asnTrafficRaw, err := m.trafficMonitor.FetchASNTrafficFromCloudflare(ctx, m.config.IranASNs)
	if err != nil {
		log.Printf("⚠️  Failed to fetch ASN traffic data: %v", err)
		// Don't set asnTrafficList - will be nil/empty, chart will be skipped
	} else if len(asnTrafficRaw) > 0 {
		log.Printf("✅ Fetched ASN traffic data for %d ASNs, generating chart...", len(asnTrafficRaw))
		// Generate ASN traffic chart
		asnChartBuffer, err := GenerateASNTrafficChart(asnTrafficRaw)
		if err != nil {
			log.Printf("⚠️  Failed to generate ASN traffic chart: %v", err)
			asnChartBuffer = nil
		} else {
			log.Printf("✅ ASN traffic chart generated successfully (buffer size: %d bytes)", asnChartBuffer.Len())
		}
		
		// Add chart buffer to each ASN traffic data item (all items share the same chart)
		for _, item := range asnTrafficRaw {
			item.ChartBuffer = asnChartBuffer
			asnTrafficList = append(asnTrafficList, item)
		}
	} else {
		log.Printf("⚠️  ASN traffic data is empty (no matching ASNs or no data available)")
	}

	m.results = &models.MonitoringResult{
		Timestamp:    time.Now(),
		ASNStatuses:  asnStatuses,
		DNSStatuses:  dnsStatuses,
		TrafficData:  trafficModelData,
		ASTrafficData: asnTrafficList,
	}
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.bgpClient != nil {
		m.bgpClient.Stop()
	}
}

