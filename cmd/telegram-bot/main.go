package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	// Start bot
	go bot.Start(ctx)

	// Start periodic updates
	go bot.SendPeriodicUpdates(ctx)

	log.Println("NetBlocks Telegram Bot started. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
}

