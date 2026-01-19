package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
	"github.com/netblocks/netblocks/internal/monitor"
	"github.com/netblocks/netblocks/internal/telegram"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Check for Telegram token
	if cfg.TelegramToken == "" {
		token := os.Getenv("TELEGRAM_BOT_TOKEN")
		if token == "" {
			log.Fatal("Telegram bot token not found. Set TELEGRAM_BOT_TOKEN environment variable or add it to config.json")
		}
		cfg.TelegramToken = token
		log.Println("✓ Telegram token loaded from environment variable")
	}

	// Check for Telegram channel from environment variable
	if cfg.TelegramChannel == "" {
		channel := os.Getenv("TELEGRAM_CHANNEL")
		if channel != "" {
			cfg.TelegramChannel = channel
			log.Printf("✓ Telegram channel loaded from environment variable: %s", channel)
		}
	}
	
	// Load Cloudflare credentials from environment variables (GitHub secrets)
	// Environment variables take precedence for bot deployment
	if token := os.Getenv("CLOUDFLARE_TOKEN"); token != "" {
		cfg.CloudflareToken = token
		log.Println("✓ Cloudflare token loaded from environment variable (GitHub secret)")
	}
	
	if email := os.Getenv("CLOUDFLARE_EMAIL"); email != "" {
		cfg.CloudflareEmail = email
		log.Println("✓ Cloudflare email loaded from environment variable (GitHub secret)")
	}
	
	if key := os.Getenv("CLOUDFLARE_KEY"); key != "" {
		cfg.CloudflareKey = key
		log.Println("✓ Cloudflare key loaded from environment variable (GitHub secret)")
	}
	
	// Log if Cloudflare credentials are available (for ASN traffic chart)
	if cfg.CloudflareToken != "" || (cfg.CloudflareEmail != "" && cfg.CloudflareKey != "") {
		log.Println("✓ Cloudflare credentials available - ASN traffic chart will be generated")
	} else {
		log.Println("⚠️  No Cloudflare credentials found - ASN traffic chart will be skipped")
	}

	// Create monitor
	mon, err := monitor.NewMonitor(cfg)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}
	defer mon.Stop()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Perform initial check to ensure DNS results are available before bot starts
	// This also fetches Cloudflare data FIRST before anything else
	log.Println("🔄 Performing initial checks (Cloudflare, DNS, BGP)...")
	log.Println("📊 Fetching Cloudflare Radar data FIRST...")
	mon.PerformInitialCheck(ctx)
	log.Println("✅ Initial checks completed - Cloudflare data fetched")

	// Create Telegram bot
	bot, err := telegram.NewBot(cfg.TelegramToken, cfg, func() (*models.MonitoringResult, error) {
		result := mon.GetResults()
		return result, nil
	})
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Start monitor in background
	go mon.Start(ctx)

	// Start periodic updates in background
	go bot.SendPeriodicUpdates(ctx)

	log.Println("✅ NetBlocks Telegram Bot started successfully!")
	log.Println("📊 Monitoring Iranian ASNs and DNS servers...")
	log.Println("🤖 Bot is ready to receive commands")
	if cfg.TelegramChannel != "" {
		log.Printf("📢 Channel updates enabled for: %s", cfg.TelegramChannel)
		log.Println("   Channel will receive updates every 10 minutes")
		
		// Send startup message to channel
		go bot.SendStartupMessage(ctx)
	}
	log.Println("")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Handle signals in a goroutine
	go func() {
		<-sigChan
		log.Println("")
		log.Println("Received shutdown signal, shutting down gracefully...")
		cancel()
	}()

	// Start bot - this blocks and keeps the process alive
	// Bot will stop when context is cancelled (by signal handler or error)
	bot.Start(ctx)
	
	// Give goroutines time to clean up
	log.Println("Bot stopped, cleaning up...")
	time.Sleep(1 * time.Second)
	log.Println("Shutdown complete.")
}
