package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
	"github.com/netblocks/netblocks/internal/monitor"
	"github.com/netblocks/netblocks/internal/telegram"
)

func main() {
	startTime := time.Now()
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

	// Error channels for goroutine error reporting
	monitorErrChan := make(chan error, 1)
	botErrChan := make(chan error, 1)
	updatesErrChan := make(chan error, 1)

	// Use WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Start monitor with panic recovery
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("❌ PANIC in monitor goroutine: %v", r)
				monitorErrChan <- fmt.Errorf("panic in monitor: %v", r)
			}
		}()
		log.Println("✅ Starting monitor goroutine...")
		mon.Start(ctx)
		log.Println("⚠️ Monitor goroutine stopped")
	}()

	// Start bot with panic recovery
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("❌ PANIC in bot goroutine: %v", r)
				botErrChan <- fmt.Errorf("panic in bot: %v", r)
			}
		}()
		log.Println("🚀 Starting Telegram bot goroutine...")
		bot.Start(ctx)
		log.Println("⚠️ Bot goroutine stopped")
	}()

	// Start periodic updates with panic recovery
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("❌ PANIC in periodic updates goroutine: %v", r)
				updatesErrChan <- fmt.Errorf("panic in periodic updates: %v", r)
			}
		}()
		log.Println("🔄 Starting periodic updates goroutine...")
		bot.SendPeriodicUpdates(ctx)
		log.Println("⚠️ Periodic updates goroutine stopped")
	}()

	// Give components time to initialize
	log.Println("⏳ Waiting for components to initialize...")
	time.Sleep(5 * time.Second)

	// Startup verification
	log.Println("")
	log.Println("✅ NetBlocks Telegram Bot started successfully!")
	log.Printf("📊 Monitoring %d ASNs and %d+ DNS servers", len(cfg.IranASNs), len(cfg.DNSServers))
	log.Println("🤖 Bot is ready to receive commands")
	log.Printf("🆔 Process ID: %d", os.Getpid())
	if cfg.TelegramChannel != "" {
		log.Printf("📢 Channel updates enabled for: %s", cfg.TelegramChannel)
		log.Println("   Channel will receive updates every 10 minutes")
		
		// Send immediate startup message to channel
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("❌ PANIC in startup message: %v", r)
				}
			}()
			bot.SendStartupMessage(ctx)
		}()
	}
	log.Println("")
	log.Println("🔄 Bot is running continuously...")
	log.Println("✅ OK - Bot is running in background")
	log.Println("")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Heartbeat ticker - logs every 5 minutes to show process is alive
	heartbeat := time.NewTicker(5 * time.Minute)
	defer heartbeat.Stop()

	// Main loop with heartbeat and error monitoring
	log.Println("💓 Heartbeat started - process will log status every 5 minutes")
	for {
		select {
		case sig := <-sigChan:
			log.Printf("📥 Received shutdown signal: %v", sig)
			log.Println("🛑 Shutting down gracefully...")
			
			// Cancel context to signal all goroutines to stop
			cancel()
			
			// Wait for goroutines to finish (with timeout)
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			
			select {
			case <-done:
				log.Println("✅ All goroutines stopped cleanly")
			case <-time.After(10 * time.Second):
				log.Println("⚠️ Timeout waiting for goroutines to stop")
			}
			
			log.Println("✅ Shutdown complete.")
			return
			
		case <-ctx.Done():
			log.Println("🛑 Context cancelled, shutting down...")
			wg.Wait()
			log.Println("✅ Shutdown complete.")
			return
			
		case err := <-monitorErrChan:
			log.Printf("⚠️ Error in monitor goroutine: %v", err)
			// Don't exit, just log the error
			
		case err := <-botErrChan:
			log.Printf("⚠️ Error in bot goroutine: %v", err)
			// Don't exit, just log the error
			
		case err := <-updatesErrChan:
			log.Printf("⚠️ Error in periodic updates goroutine: %v", err)
			// Don't exit, just log the error
			
		case <-heartbeat.C:
			// Periodic heartbeat to show process is alive
			uptime := time.Since(startTime)
			log.Printf("💓 Bot heartbeat - still running (PID: %d, Uptime: %s)", 
				os.Getpid(), uptime.Round(time.Second))
			log.Printf("📊 Status: Context active=%t", ctx.Err() == nil)
		}
	}
}

