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

	// Start monitor
	go mon.Start(ctx)
	log.Println("✅ Monitor started")

	// Start bot (this blocks, so run in goroutine)
	go func() {
		log.Println("🚀 Starting Telegram bot...")
		bot.Start(ctx)
		log.Println("⚠️ Bot stopped!")
	}()

	// Start periodic updates
	go func() {
		log.Println("🔄 Starting periodic updates...")
		bot.SendPeriodicUpdates(ctx)
		log.Println("⚠️ Periodic updates stopped!")
	}()

	// Give bot time to initialize
	time.Sleep(3 * time.Second)

	log.Println("✅ NetBlocks Telegram Bot started successfully!")
	log.Println("📊 Monitoring Iranian ASNs and DNS servers...")
	log.Println("🤖 Bot is ready to receive commands")
	if cfg.TelegramChannel != "" {
		log.Printf("📢 Channel updates enabled for: %s", cfg.TelegramChannel)
		log.Println("   Channel will receive updates every 10 minutes")
		
		// Send immediate startup message to channel
		go bot.SendStartupMessage(ctx)
	}
	log.Println("")
	log.Println("🔄 Bot is running continuously...")
	log.Println("✅ OK - Bot is running in background")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Keep the main process alive - block on signal channel
	// This keeps the process running in the background
	<-sigChan
	log.Printf("Received shutdown signal")
	log.Println("Shutting down gracefully...")
	cancel()
	// Give goroutines time to clean up
	time.Sleep(2 * time.Second)
	log.Println("Shutdown complete.")
}

