package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
	"github.com/netblocks/netblocks/internal/monitor"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	outputDir := flag.String("output", ".", "Directory to save chart images (default: current directory)")
	saveCharts := flag.Bool("charts", false, "Save traffic charts as PNG files")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Check if Cloudflare credentials are available in config file
	// CLI reads from config.json (not environment variables, unlike bot)
	if cfg.CloudflareToken != "" {
		log.Printf("✓ Cloudflare token loaded from config file (%d chars)", len(cfg.CloudflareToken))
	} else if cfg.CloudflareEmail != "" && cfg.CloudflareKey != "" {
		log.Printf("✓ Cloudflare API key loaded from config file (email: %s)", cfg.CloudflareEmail)
	} else {
		log.Println("⚠️  No Cloudflare credentials found in config file - traffic charts will be skipped")
		log.Println("   Add 'cloudflare_token' to your config.json to enable traffic charts")
	}

	// Create monitor
	mon, err := monitor.NewMonitor(cfg)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}
	defer mon.Stop()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Default behavior: run once and exit
	// Perform initial check synchronously to ensure DNS results are available
	mon.PerformInitialCheck(ctx)
	
	// Start monitor briefly to allow BGP updates to arrive
	go mon.Start(ctx)
	time.Sleep(5 * time.Second) // Give BGP a moment to receive some updates
	
	// Get results
	result := mon.GetResults()
	
	// Print status and exit (default behavior: run once)
	printStatus(result)
	
	// Save charts if requested
	if *saveCharts {
		saveChartsToFiles(result, *outputDir)
	}
}

func printStatus(result *models.MonitoringResult) {
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Printf("📊 NetBlocks Monitoring Status - %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("═", 80))

	// ASN Status
	fmt.Println("\n🌐 ASN Connectivity")
	fmt.Println(strings.Repeat("─", 80))
	connectedCount := 0
	totalCount := len(result.ASNStatuses)

	// Sort ASNs for better readability (connected first)
	type asnEntry struct {
		asn      string
		status   *models.ASNStatus
		connected bool
	}
	var entries []asnEntry
	for asn, status := range result.ASNStatuses {
		entries = append(entries, asnEntry{asn: asn, status: status, connected: status.Connected})
		if status.Connected {
			connectedCount++
		}
	}
	
	// Sort: connected first, then by ASN
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].connected != entries[j].connected {
				if !entries[i].connected && entries[j].connected {
					entries[i], entries[j] = entries[j], entries[i]
				}
			} else if entries[i].asn > entries[j].asn {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	for _, entry := range entries {
		statusIcon := "🔴"
		if entry.status.Connected {
			statusIcon = "🟢"
		}
		lastSeen := "Never"
		if !entry.status.LastSeen.IsZero() {
			lastSeen = entry.status.LastSeen.Format("2006-01-02 15:04:05")
		}
		// Display ASN with readable name if available
		asnDisplay := entry.asn
		if entry.status.Name != "" {
			asnDisplay = fmt.Sprintf("%s - %s", entry.asn, entry.status.Name)
		}
		fmt.Printf("%s %-50s Last seen: %s\n", statusIcon, asnDisplay, lastSeen)
	}

	fmt.Printf("\n📈 Summary: %d/%d Connected\n", connectedCount, totalCount)

	// DNS Status
	fmt.Println("\n🔍 DNS Servers")
	fmt.Println(strings.Repeat("─", 80))
	aliveCount := 0
	dnsTotal := len(result.DNSStatuses)

	// Sort DNS servers (alive first)
	type dnsEntry struct {
		addr    string
		status  *models.DNSStatus
		alive   bool
	}
	var dnsEntries []dnsEntry
	for addr, status := range result.DNSStatuses {
		dnsEntries = append(dnsEntries, dnsEntry{addr: addr, status: status, alive: status.Alive})
		if status.Alive {
			aliveCount++
		}
	}
	
	// Sort: alive first, then by name
	for i := 0; i < len(dnsEntries)-1; i++ {
		for j := i + 1; j < len(dnsEntries); j++ {
			if dnsEntries[i].alive != dnsEntries[j].alive {
				if !dnsEntries[i].alive && dnsEntries[j].alive {
					dnsEntries[i], dnsEntries[j] = dnsEntries[j], dnsEntries[i]
				}
			} else if dnsEntries[i].status.Name > dnsEntries[j].status.Name {
				dnsEntries[i], dnsEntries[j] = dnsEntries[j], dnsEntries[i]
			}
		}
	}

	for _, entry := range dnsEntries {
		statusIcon := "🔴"
		if entry.status.Alive {
			statusIcon = "🟢"
		}
		responseTime := entry.status.ResponseTime.Milliseconds()
		fmt.Printf("%s %-45s %-18s %dms", statusIcon, entry.status.Name, entry.addr, responseTime)
		if entry.status.Error != "" {
			fmt.Printf(" ⚠️  %s", entry.status.Error)
		}
		fmt.Println()
	}

	fmt.Printf("\n📈 Summary: %d/%d Alive\n", aliveCount, dnsTotal)
	fmt.Println()
}

// saveChartsToFiles saves traffic charts as PNG files
func saveChartsToFiles(result *models.MonitoringResult, outputDir string) {
	timestamp := result.Timestamp.Format("20060102_150405")
	
	// Save Iran traffic charts
	if result.TrafficData != nil && result.TrafficData.ChartBuffer != nil && result.TrafficData.ChartBuffer.Len() > 0 {
		filename := fmt.Sprintf("%s/iran_traffic_%s.png", outputDir, timestamp)
		if err := os.WriteFile(filename, result.TrafficData.ChartBuffer.Bytes(), 0644); err != nil {
			log.Printf("⚠️  Failed to save Iran traffic chart: %v", err)
		} else {
			fmt.Printf("\n✅ Iran 24h traffic chart saved: %s\n", filename)
		}
	} else {
		fmt.Printf("\n⚠️  Iran 24h traffic chart not available\n")
	}

	if result.TrafficData != nil && result.TrafficData.Chart7dBuffer != nil && result.TrafficData.Chart7dBuffer.Len() > 0 {
		filename := fmt.Sprintf("%s/iran_traffic_7d_%s.png", outputDir, timestamp)
		if err := os.WriteFile(filename, result.TrafficData.Chart7dBuffer.Bytes(), 0644); err != nil {
			log.Printf("⚠️  Failed to save Iran 7d traffic chart: %v", err)
		} else {
			fmt.Printf("✅ Iran 7d traffic chart saved: %s\n", filename)
		}
	}
	
	// Save ASN traffic chart
	if result.ASTrafficData != nil && len(result.ASTrafficData) > 0 {
		firstItem := result.ASTrafficData[0]
		if firstItem.ChartBuffer != nil && firstItem.ChartBuffer.Len() > 0 {
			filename := fmt.Sprintf("%s/asn_traffic_%s.png", outputDir, timestamp)
			if err := os.WriteFile(filename, firstItem.ChartBuffer.Bytes(), 0644); err != nil {
				log.Printf("⚠️  Failed to save ASN traffic chart: %v", err)
			} else {
				fmt.Printf("✅ ASN traffic chart saved: %s\n", filename)
			}
		} else {
			fmt.Printf("⚠️  ASN traffic chart not available\n")
		}
	} else {
		fmt.Printf("⚠️  ASN traffic chart not available\n")
	}
}

