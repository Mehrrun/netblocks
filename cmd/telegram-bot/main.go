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
	
	// Load Cloudflare credentials from environment variables (preferred for security)
	// Supports both API Token (preferred) and API Key (legacy)
	if token := os.Getenv("CLOUDFLARE_TOKEN"); token != "" {
		cfg.CloudflareToken = token
		log.Printf("✅ Cloudflare API Token loaded from CLOUDFLARE_TOKEN (length: %d chars)", len(token))
	}
	
	if email := os.Getenv("CLOUDFLARE_EMAIL"); email != "" {
		cfg.CloudflareEmail = email
		log.Printf("✅ Cloudflare email loaded from CLOUDFLARE_EMAIL: %s", email)
	}
	
	if key := os.Getenv("CLOUDFLARE_KEY"); key != "" {
		cfg.CloudflareKey = key
		log.Printf("✅ Cloudflare API key loaded from CLOUDFLARE_KEY (length: %d chars)", len(key))
	}
	
	// Warn if Cloudflare credentials are missing
	if cfg.CloudflareToken == "" && (cfg.CloudflareEmail == "" || cfg.CloudflareKey == "") {
		log.Println("⚠️  ========================================")
		log.Println("⚠️  CLOUDFLARE CREDENTIALS NOT CONFIGURED")
		log.Println("⚠️  ========================================")
		log.Println("⚠️  Traffic charts will NOT be available.")
		log.Println("⚠️  Set CLOUDFLARE_TOKEN environment variable (recommended)")
		log.Println("⚠️  OR set CLOUDFLARE_EMAIL and CLOUDFLARE_KEY (legacy method)")
		log.Println("⚠️  ========================================")
	} else {
		log.Println("✅ ========================================")
		log.Println("✅ CLOUDFLARE CREDENTIALS CONFIGURED")
		log.Println("✅ Traffic monitoring ENABLED")
		if cfg.CloudflareToken != "" {
			log.Println("✅ Using API Token authentication (recommended)")
		} else {
			log.Println("✅ Using API Key authentication (legacy)")
		}
		log.Println("✅ ========================================")
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
	mon.PerformInitialCheck(ctx)

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
