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
	bgpClient, err := NewRISLiveClient(cfg.RISLiveURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create RIS Live client: %w", err)
	}

	for _, asn := range cfg.IranASNs {
		if err := bgpClient.SubscribeToASN(asn); err != nil {
			log.Printf("Warning: Failed to subscribe to ASN %s: %v", asn, err)
		}
	}

	bgpClient.Start()

	dnsMonitor := NewDNSMonitor(cfg.DNSServers, 8*time.Second)
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
func (m *Monitor) PerformInitialCheck(ctx context.Context) {
	log.Println("📡 Fetching Cloudflare Radar data for Iran...")
	trafficData, err := m.trafficMonitor.FetchFromCloudflare(ctx)
	if err != nil {
		log.Printf("⚠️  Cloudflare fetch error (will use defaults): %v", err)
	} else if trafficData != nil {
		log.Printf("✅ Cloudflare 24h data fetched - Current: %s, Status: %s %s",
			formatAbsoluteValue(trafficData.CurrentLevel, trafficData.Unit),
			trafficData.StatusEmoji, trafficData.Status)
	} else {
		log.Println("⚠️  Cloudflare data is nil (will use defaults)")
	}

	if _, err := m.trafficMonitor.Fetch7dFromCloudflare(ctx); err != nil {
		log.Printf("⚠️  Cloudflare 7d fetch error: %v", err)
	} else {
		log.Println("✅ Cloudflare 7d data fetched")
	}

	log.Println("🔍 Checking DNS servers...")
	_ = m.dnsMonitor.CheckAll(ctx)

	time.Sleep(1 * time.Second)
	m.updateResults(ctx)
}

// Start starts monitoring
func (m *Monitor) Start(ctx context.Context) {
	go m.dnsMonitor.StartPeriodicCheck(ctx, m.config.Interval)
	go m.trafficMonitor.Start(ctx)

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

	trafficData, err := m.trafficMonitor.GetTrafficData(ctx)
	if err != nil {
		log.Printf("⚠️  Traffic data fetch error: %v", err)
	}

	var trafficModelData *models.TrafficData
	if trafficData != nil {
		chartBuffer, chartErr := GenerateTrafficChart(trafficData)
		if chartErr != nil {
			log.Printf("⚠️  Failed to generate 24h traffic chart: %v", chartErr)
			chartBuffer = nil
		}

		trafficModelData = &models.TrafficData{
			CurrentLevel:  trafficData.CurrentLevel,
			Baseline:      trafficData.Baseline,
			Trend24h:      trafficData.Trend24h,
			Timestamps:    trafficData.Timestamps,
			ChangePercent: trafficData.ChangePercent,
			Status:        trafficData.Status,
			StatusEmoji:   trafficData.StatusEmoji,
			Unit:          trafficData.Unit,
			Source:        trafficData.Source,
			ChartBuffer:   chartBuffer,
			LastUpdate:    trafficData.LastUpdate,
		}
	}

	// Attach 7d chart when available
	traffic7d, err7d := m.trafficMonitor.GetTrafficData7d(ctx)
	if err7d != nil {
		log.Printf("⚠️  7d traffic data fetch error: %v", err7d)
	} else if traffic7d != nil && trafficModelData != nil {
		chart7d, chartErr := GenerateTrafficChart7d(traffic7d)
		if chartErr != nil {
			log.Printf("⚠️  Failed to generate 7d traffic chart: %v", chartErr)
		} else {
			trafficModelData.Trend7d = traffic7d.Trend7d
			trafficModelData.Timestamps7d = traffic7d.Timestamps7d
			trafficModelData.Chart7dBuffer = chart7d
		}
	} else if traffic7d != nil && trafficModelData == nil {
		// 7d available but 24h missing — still expose 7d as primary-ish model
		chart7d, chartErr := GenerateTrafficChart7d(traffic7d)
		if chartErr != nil {
			log.Printf("⚠️  Failed to generate 7d traffic chart: %v", chartErr)
			chart7d = nil
		}
		trafficModelData = &models.TrafficData{
			CurrentLevel:  traffic7d.CurrentLevel,
			Baseline:      traffic7d.Baseline,
			Trend7d:       traffic7d.Trend7d,
			Timestamps7d:  traffic7d.Timestamps7d,
			ChangePercent: traffic7d.ChangePercent,
			Status:        traffic7d.Status,
			StatusEmoji:   traffic7d.StatusEmoji,
			Unit:          traffic7d.Unit,
			Source:        traffic7d.Source,
			Chart7dBuffer: chart7d,
			LastUpdate:    traffic7d.LastUpdate,
		}
	}

	var asnTrafficList []*models.ASTrafficData
	asnTrafficRaw, err := m.trafficMonitor.FetchASNTrafficFromCloudflare(ctx)
	if err != nil {
		log.Printf("⚠️  Failed to fetch ASN traffic data: %v", err)
	} else if len(asnTrafficRaw) > 0 {
		log.Printf("✅ Fetched ASN traffic data for %d ASNs, generating chart...", len(asnTrafficRaw))
		asnChartBuffer, chartErr := GenerateASNTrafficChart(asnTrafficRaw)
		if chartErr != nil {
			log.Printf("⚠️  Failed to generate ASN traffic chart: %v", chartErr)
			asnChartBuffer = nil
		} else {
			log.Printf("✅ ASN traffic chart generated successfully (buffer size: %d bytes)", asnChartBuffer.Len())
		}
		for _, item := range asnTrafficRaw {
			item.ChartBuffer = asnChartBuffer
			asnTrafficList = append(asnTrafficList, item)
		}
	} else {
		log.Printf("⚠️  ASN traffic data is empty")
	}

	m.results = &models.MonitoringResult{
		Timestamp:     time.Now(),
		ASNStatuses:   asnStatuses,
		DNSStatuses:   dnsStatuses,
		TrafficData:   trafficModelData,
		ASTrafficData: asnTrafficList,
	}
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.bgpClient != nil {
		m.bgpClient.Stop()
	}
}
