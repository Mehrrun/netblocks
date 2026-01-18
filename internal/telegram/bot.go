package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
	"github.com/netblocks/netblocks/internal/monitor"
)

// Bot represents the Telegram bot
type Bot struct {
	api            *tgbotapi.BotAPI
	config         *config.Config
	updateInterval time.Duration
	intervalMu     sync.RWMutex   // Mutex for updateInterval
	onStatusUpdate func() (*models.MonitoringResult, error)
	subscribedChats map[int64]bool // Track users who have interacted with the bot
	chatsMu         sync.RWMutex   // Mutex for subscribedChats
	channelID       string         // Channel username or ID for periodic updates
}

// NewBot creates a new Telegram bot
func NewBot(token string, cfg *config.Config, onStatusUpdate func() (*models.MonitoringResult, error)) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("telegram bot token is empty")
	}
	
	log.Printf("🔑 Initializing Telegram bot with token: %s...", token[:10]+"...")
	
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API client: %w", err)
	}

	// Test the connection by getting bot info
	botInfo, err := api.GetMe()
	if err != nil {
		return nil, fmt.Errorf("failed to verify bot token (GetMe failed): %w", err)
	}

	log.Printf("✅ Successfully authorized as bot: @%s (ID: %d, Name: %s)", 
		botInfo.UserName, botInfo.ID, botInfo.FirstName)

	// Default to 10 minutes if not set
	updateInterval := cfg.Interval
	if updateInterval == 0 {
		updateInterval = 10 * time.Minute
	}

	// Normalize channel ID/username format
	channelID := cfg.TelegramChannel
	if channelID != "" {
		// Handle t.me/channelname format -> @channelname
		if strings.HasPrefix(channelID, "t.me/") {
			channelID = "@" + strings.TrimPrefix(channelID, "t.me/")
		}
		// Ensure it starts with @ if it's a username
		if !strings.HasPrefix(channelID, "@") && !strings.HasPrefix(channelID, "-") {
			// If it doesn't start with @ or - (negative chat ID), assume it's a username
			channelID = "@" + channelID
		}
		log.Printf("📢 Channel configured: %s", channelID)
	} else {
		log.Printf("⚠️  No channel configured - channel updates disabled")
	}

	bot := &Bot{
		api:              api,
		config:           cfg,
		updateInterval:   updateInterval,
		onStatusUpdate:   onStatusUpdate,
		subscribedChats:  make(map[int64]bool),
		channelID:        channelID,
	}

	log.Printf("✅ Bot initialized successfully")
	return bot, nil
}

// SendStartupMessage sends a startup notification to the channel
func (b *Bot) SendStartupMessage(ctx context.Context) {
	if b.channelID == "" {
		return
	}
	
	startupMsg := fmt.Sprintf("🚀 *NetBlocks Bot Started*\n\n✅ Bot is now monitoring Iranian networks\n📊 Monitoring %d ASNs and %d+ DNS servers\n⏰ Updates will be sent every 10 minutes\n\nBot started at: `%s`",
		len(b.config.IranASNs),
		len(b.config.DNSServers),
		time.Now().Format("2006-01-02 15:04:05"))
	
	log.Printf("📤 Sending startup message to channel: %s", b.channelID)
	b.sendMessage(b.channelID, startupMsg)
}

// Start starts the bot
func (b *Bot) Start(ctx context.Context) {
	log.Println("🤖 Starting Telegram bot update handler...")
	
	// Delete any pending webhook to ensure we use long polling
	deleteWebhookConfig := tgbotapi.DeleteWebhookConfig{
		DropPendingUpdates: true,
	}
	_, err := b.api.Request(deleteWebhookConfig)
	if err != nil {
		log.Printf("⚠️ Warning: Failed to delete webhook (may not exist): %v", err)
	} else {
		log.Println("✅ Cleared any existing webhooks, using long polling")
	}
	
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	log.Println("📡 Connecting to Telegram API for updates...")
	updates := b.api.GetUpdatesChan(u)
	log.Println("✅ Telegram bot update channel initialized successfully!")
	log.Println("⏳ Waiting for incoming messages...")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Bot context cancelled, stopping update handler...")
			return
		case update := <-updates:
			if update.Message == nil {
				// Handle callback queries (button presses) if needed
				if update.CallbackQuery != nil {
					log.Printf("📥 Received callback query from user %d", update.CallbackQuery.From.ID)
					// You can add callback handling here if needed
				}
				continue
			}

			log.Printf("📥 Received message from user %d (@%s): %s", 
				update.Message.From.ID, 
				update.Message.From.UserName,
				update.Message.Text)
			
			// Handle message in a goroutine to avoid blocking
			go b.handleMessage(update.Message)
		}
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	// Add user to subscribed chats when they interact with the bot
	b.addSubscribedChat(msg.Chat.ID)
	
	// Handle empty messages
	if msg.Text == "" {
		log.Printf("⚠️ Received message with empty text from user %d", msg.Chat.ID)
		return
	}
	
	command := strings.ToLower(strings.TrimSpace(msg.Text))
	log.Printf("🔍 Processing command: %s", command)
	
	switch {
	case strings.HasPrefix(command, "/start"):
		log.Println("📤 Sending welcome message...")
		b.sendWelcome(msg.Chat.ID)
	case strings.HasPrefix(command, "/status"):
		log.Println("📤 Sending status update...")
		b.sendStatus(msg.Chat.ID)
	case strings.HasPrefix(command, "/interval"):
		parts := strings.Fields(command)
		if len(parts) > 1 {
			log.Printf("📤 Setting interval to %s minutes...", parts[1])
			b.handleSetInterval(msg.Chat.ID, parts[1])
		} else {
			b.sendMessage(msg.Chat.ID, "Usage: /interval <minutes>\nExample: /interval 5")
		}
	case strings.HasPrefix(command, "/testchannel"):
		log.Println("📤 Testing channel...")
		b.handleTestChannel(msg.Chat.ID)
	case strings.HasPrefix(command, "/help"):
		log.Println("📤 Sending help message...")
		b.sendHelp(msg.Chat.ID)
	default:
		log.Printf("❓ Unknown command: %s", command)
		b.sendMessage(msg.Chat.ID, "Unknown command. Use /help to see available commands.")
	}
}

// addSubscribedChat adds a chat ID to the subscribed chats list
func (b *Bot) addSubscribedChat(chatID int64) {
	b.chatsMu.Lock()
	defer b.chatsMu.Unlock()
	b.subscribedChats[chatID] = true
}

// getSubscribedChats returns a copy of all subscribed chat IDs
func (b *Bot) getSubscribedChats() []int64 {
	b.chatsMu.RLock()
	defer b.chatsMu.RUnlock()
	
	chats := make([]int64, 0, len(b.subscribedChats))
	for chatID := range b.subscribedChats {
		chats = append(chats, chatID)
	}
	return chats
}

func (b *Bot) sendWelcome(chatID int64) {
	intervalMinutes := int(b.getUpdateInterval().Minutes())
	
	text := fmt.Sprintf(`🤖 Welcome to NetBlocks Monitor Bot!

I monitor:
• Iranian AS (Autonomous Systems) connectivity via BGP
• Iranian DNS servers availability

Commands:
/status - Get current monitoring status
/interval <minutes> - Set periodic update interval
/help - Show help message

You will receive automatic updates every %d minutes. Use /interval to change this.`, intervalMinutes)
	
	b.sendMessage(chatID, text)
}

func (b *Bot) sendHelp(chatID int64) {
	text := `📖 NetBlocks Monitor Bot Commands:

/start - Start the bot and see welcome message
/status - Get current status of all monitored systems
/interval <minutes> - Set monitoring check interval (e.g., /interval 5)
/testchannel - Test sending a message to the configured channel
/help - Show this help message

Example:
/interval 10 - Set interval to 10 minutes`
	
	b.sendMessage(chatID, text)
}

func (b *Bot) handleTestChannel(chatID int64) {
	if b.channelID == "" {
		b.sendMessage(chatID, "❌ No channel configured. Set `telegram_channel` in config.json")
		return
	}
	
	testMsg := fmt.Sprintf("🧪 *Test Message*\n\nThis is a test message sent to channel: `%s`\n\nIf you see this in the channel, the bot is working correctly!", b.channelID)
	
	log.Printf("🧪 Testing channel send to: %s", b.channelID)
	log.Printf("📋 Channel ID format: %T, value: %v", b.channelID, b.channelID)
	
	// Try sending to channel
	b.sendMessage(b.channelID, testMsg)
	
	// Also send confirmation to user
	b.sendMessage(chatID, fmt.Sprintf("✅ Test message sent to channel: %s\n\n⚠️ If you don't see it in the channel:\n1. Make sure the bot is an administrator\n2. Bot must have 'Post messages' permission\n3. Check bot logs for errors", b.channelID))
}

func (b *Bot) handleSetInterval(chatID int64, intervalStr string) {
	minutes, err := strconv.Atoi(intervalStr)
	if err != nil || minutes < 1 {
		b.sendMessage(chatID, "❌ Invalid interval. Please provide a number of minutes (minimum 1).")
		return
	}

	newInterval := time.Duration(minutes) * time.Minute
	
	b.intervalMu.Lock()
	b.updateInterval = newInterval
	b.intervalMu.Unlock()
	
	b.config.Interval = newInterval
	
	// Save config
	if err := config.SaveConfig("config.json", b.config); err != nil {
		log.Printf("Failed to save config: %v", err)
	}

	b.sendMessage(chatID, fmt.Sprintf("✅ Periodic update interval set to %d minutes. You will receive updates every %d minutes.", minutes, minutes))
}

// getUpdateInterval safely gets the current update interval
func (b *Bot) getUpdateInterval() time.Duration {
	b.intervalMu.RLock()
	defer b.intervalMu.RUnlock()
	interval := b.updateInterval
	if interval == 0 {
		interval = 10 * time.Minute
	}
	return interval
}

func (b *Bot) sendStatus(chatID int64) {
	if b.onStatusUpdate == nil {
		b.sendMessage(chatID, "❌ Status update function not available")
		return
	}

	result, err := b.onStatusUpdate()
	if err != nil {
		b.sendMessage(chatID, fmt.Sprintf("❌ Error getting status: %v", err))
		return
	}

	// Split status into multiple messages to avoid Telegram's 4096 character limit
	b.sendStatusMessages(chatID, result)
}

// formatStatus formats the complete status (for logging)
func (b *Bot) formatStatus(result *models.MonitoringResult) string {
	var builder strings.Builder
	
	builder.WriteString("📊 NetBlocks Monitoring Status\n")
	builder.WriteString(fmt.Sprintf("⏰ Last Update: %s\n\n", result.Timestamp.Format("2006-01-02 15:04:05")))
	
	// ASN Status
	asnText := b.formatASNStatus(result)
	builder.WriteString(asnText)
	builder.WriteString("\n")
	
	// DNS Status
	dnsText := b.formatDNSStatus(result)
	builder.WriteString(dnsText)
	
	return builder.String()
}

// formatASNStatus formats ASN connectivity status
func (b *Bot) formatASNStatus(result *models.MonitoringResult) string {
	var builder strings.Builder
	
	builder.WriteString("🌐 *ASN Connectivity*\n")
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	connectedCount := 0
	totalCount := len(result.ASNStatuses)
	
	// Sort ASNs for better readability (connected first, then by name)
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
		icon := "🔴"
		if entry.status.Connected {
			icon = "🟢"
		}
		lastSeen := "Never"
		if !entry.status.LastSeen.IsZero() {
			lastSeen = entry.status.LastSeen.Format("15:04:05")
		}
		// Display ASN with readable name if available
		asnDisplay := entry.asn
		if entry.status.Name != "" {
			asnDisplay = fmt.Sprintf("%s - %s", entry.asn, entry.status.Name)
		}
		builder.WriteString(fmt.Sprintf("%s `%s`\n   └─ Last seen: %s\n", icon, asnDisplay, lastSeen))
	}
	
	builder.WriteString(fmt.Sprintf("\n📈 *Summary:* %d/%d Connected\n", connectedCount, totalCount))
	
	return builder.String()
}

// formatDNSStatus formats DNS server status
func (b *Bot) formatDNSStatus(result *models.MonitoringResult) string {
	var builder strings.Builder
	
	builder.WriteString("🔍 *DNS Servers*\n")
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
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
		icon := "🔴"
		if entry.status.Alive {
			icon = "🟢"
		}
		responseTime := entry.status.ResponseTime.Milliseconds()
		builder.WriteString(fmt.Sprintf("%s *%s*\n   └─ `%s` - %dms\n", 
			icon, entry.status.Name, entry.addr, responseTime))
		if entry.status.Error != "" {
			builder.WriteString(fmt.Sprintf("   └─ ⚠️ Error: %s\n", entry.status.Error))
		}
	}
	
	builder.WriteString(fmt.Sprintf("\n📈 *Summary:* %d/%d Alive\n", aliveCount, dnsTotal))
	
	return builder.String()
}

// sendMessage sends a message to a chat (user or channel)
// chatID can be an int64 for users or a string for channel username (e.g., "@channel")
func (b *Bot) sendMessage(chatID interface{}, text string) {
	const maxMessageLength = 4096
	
	// Split message if it's too long
	if len(text) <= maxMessageLength {
		var msg tgbotapi.MessageConfig
		
		// Handle both int64 (user chat ID) and string (channel username)
		switch id := chatID.(type) {
		case int64:
			msg = tgbotapi.NewMessage(id, text)
		case string:
			msg = tgbotapi.NewMessageToChannel(id, text)
		default:
			log.Printf("Error: invalid chatID type: %T", chatID)
			return
		}
		
		msg.ParseMode = tgbotapi.ModeMarkdown
		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("❌ ERROR sending message to %v: %v", chatID, err)
			// For channels, provide helpful error message
			if channelName, ok := chatID.(string); ok {
				log.Printf("⚠️  CHANNEL ERROR DETAILS:")
				log.Printf("   Channel: %v", channelName)
				log.Printf("   Error: %v", err)
				log.Printf("⚠️  TROUBLESHOOTING:")
				log.Printf("   1. Make sure the bot is added as an administrator to the channel")
				log.Printf("   2. Bot must have 'Post messages' permission")
				log.Printf("   3. If using username (@channel), try numeric channel ID (e.g., -1001234567890)")
				log.Printf("   4. Check if channel exists and is accessible")
			}
		} else {
			log.Printf("✅ Successfully sent message to %v (message ID: %d, chat ID: %d)", chatID, sentMsg.MessageID, sentMsg.Chat.ID)
		}
		return
	}
	
	// Split into chunks
	lines := strings.Split(text, "\n")
	var currentChunk strings.Builder
	chunkNum := 1
	
	for _, line := range lines {
		// Check if adding this line would exceed the limit
		potentialLength := currentChunk.Len() + len(line) + 1 // +1 for newline
		if potentialLength > maxMessageLength-50 { // Leave some margin
			// Send current chunk
			if currentChunk.Len() > 0 {
				chunkText := fmt.Sprintf("📄 *Part %d*\n\n%s", chunkNum, currentChunk.String())
				var msg tgbotapi.MessageConfig
				switch id := chatID.(type) {
				case int64:
					msg = tgbotapi.NewMessage(id, chunkText)
				case string:
					msg = tgbotapi.NewMessageToChannel(id, chunkText)
				default:
					log.Printf("Error: invalid chatID type: %T", chatID)
					continue
				}
				msg.ParseMode = tgbotapi.ModeMarkdown
				sentMsg, err := b.api.Send(msg)
				if err != nil {
					log.Printf("❌ Error sending message chunk to %v: %v", chatID, err)
				} else {
					log.Printf("✅ Sent chunk %d to %v (message ID: %d)", chunkNum, chatID, sentMsg.MessageID)
				}
				chunkNum++
				currentChunk.Reset()
			}
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
	}
	
	// Send remaining chunk
	if currentChunk.Len() > 0 {
		chunkText := fmt.Sprintf("📄 *Part %d*\n\n%s", chunkNum, currentChunk.String())
		var msg tgbotapi.MessageConfig
		switch id := chatID.(type) {
		case int64:
			msg = tgbotapi.NewMessage(id, chunkText)
		case string:
			msg = tgbotapi.NewMessageToChannel(id, chunkText)
		default:
			log.Printf("Error: invalid chatID type: %T", chatID)
			return
		}
		msg.ParseMode = tgbotapi.ModeMarkdown
		sentMsg, err := b.api.Send(msg)
		if err != nil {
			log.Printf("❌ Error sending final chunk to %v: %v", chatID, err)
		} else {
			log.Printf("✅ Sent final chunk to %v (message ID: %d)", chatID, sentMsg.MessageID)
		}
	}
}

// sendStatusMessages sends status in multiple messages
// ORDER: Header -> ASN status -> DNS status -> Traffic Chart (diagram LAST)
// chatID can be int64 (user) or string (channel username)
func (b *Bot) sendStatusMessages(chatID interface{}, result *models.MonitoringResult) {
	// Send header
	header := fmt.Sprintf("📊 *NetBlocks Monitoring Status*\n⏰ Last Update: `%s`\n", 
		result.Timestamp.Format("2006-01-02 15:04:05"))
	b.sendMessage(chatID, header)
	
	// Send ASN status (after diagram)
	asnText := b.formatASNStatus(result)
	if asnText != "" {
		b.sendMessage(chatID, asnText)
	}
	
	// Send DNS status (after diagram and ASN)
	dnsText := b.formatDNSStatus(result)
	if dnsText != "" {
		b.sendMessage(chatID, dnsText)
	}

	// Send traffic chart LAST (diagram after other data)
	if result.TrafficData != nil {
		if result.TrafficData.ChartBuffer != nil && result.TrafficData.ChartBuffer.Len() > 0 {
			log.Printf("📈 Sending traffic chart LAST (after ASN/DNS data)")
			b.sendTrafficChart(chatID, result.TrafficData)
		} else {
			log.Printf("⚠️  Traffic chart buffer is empty - skipping chart")
		}
	} else {
		log.Printf("⚠️  Traffic data is nil - no chart available")
	}
}

// SendPeriodicUpdates sends periodic status updates to all subscribed users
// Uses the interval set via /interval command (default: 10 minutes)
// Channel updates are sent every 10 minutes independently
// The interval can be changed dynamically and will take effect within 1 second
func (b *Bot) SendPeriodicUpdates(ctx context.Context) {
	// Wait a few seconds for monitoring to initialize and collect initial data
	log.Println("⏳ Waiting for initial monitoring data collection...")
	// Start immediately - monitoring data is already collected from PerformInitialCheck
	// Check every second for interval changes and time elapsed
	checkTicker := time.NewTicker(1 * time.Second)
	defer checkTicker.Stop()
	
	lastUpdateTime := time.Now()
	lastChannelUpdateTime := time.Time{} // Start with zero time so channel gets immediate update
	lastInterval := b.getUpdateInterval()
	channelInterval := 10 * time.Minute // Channel updates every 10 minutes
	
	log.Printf("Periodic updates started - will send to subscribed users every %v", lastInterval)
	if b.channelID != "" {
		log.Printf("✅ Channel updates will be sent every %v to: %s", channelInterval, b.channelID)
		log.Printf("📋 Channel will receive first status update after monitoring data is ready")
	} else {
		log.Printf("⚠️  No channel configured - skipping channel updates")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-checkTicker.C:
			currentInterval := b.getUpdateInterval()
			timeSinceLastUpdate := time.Since(lastUpdateTime)
			timeSinceLastChannelUpdate := time.Since(lastChannelUpdateTime)
			
			// Check if interval changed
			if currentInterval != lastInterval {
				log.Printf("Periodic update interval changed from %v to %v", lastInterval, currentInterval)
				lastInterval = currentInterval
				// Reset timer when interval changes so new interval takes effect immediately
				// If enough time has passed, send update now; otherwise wait for new interval
				if timeSinceLastUpdate >= currentInterval {
					lastUpdateTime = time.Time{} // Force immediate update
				} else {
					lastUpdateTime = time.Now() // Reset to wait for new interval
				}
			}
			
			// Check if it's time to send channel update (every 10 minutes)
			shouldSendChannelUpdate := false
			if b.channelID != "" {
				// If lastChannelUpdateTime is zero (startup), send immediately
				if lastChannelUpdateTime.IsZero() || timeSinceLastChannelUpdate >= channelInterval {
					shouldSendChannelUpdate = true
					if lastChannelUpdateTime.IsZero() {
						log.Printf("🚀 Sending initial channel update to: %s", b.channelID)
					} else {
						log.Printf("⏰ Channel update interval reached: %v elapsed", timeSinceLastChannelUpdate)
					}
				}
			}
			
			// Check if it's time to send user updates
			shouldSendUserUpdate := false
			if timeSinceLastUpdate >= currentInterval {
				subscribedChats := b.getSubscribedChats()
				if len(subscribedChats) > 0 {
					shouldSendUserUpdate = true
				}
			}
			
			// Perform analysis if we need to send any updates
			if shouldSendChannelUpdate || shouldSendUserUpdate {
				if b.onStatusUpdate != nil {
					result, err := b.onStatusUpdate()
					if err != nil {
						log.Printf("Error getting status for periodic update: %v", err)
						continue
					}
					
					// Send to channel if it's time (every 10 minutes)
					if shouldSendChannelUpdate {
						log.Printf("📢 Sending periodic update to channel: %s (interval: %v)", b.channelID, channelInterval)
						b.sendStatusMessages(b.channelID, result)
						lastChannelUpdateTime = time.Now()
						log.Printf("✅ Channel update sent successfully to: %s", b.channelID)
					}
					
					// Send to subscribed users if it's time
					if shouldSendUserUpdate {
						subscribedChats := b.getSubscribedChats()
						log.Printf("Sending periodic update to %d subscribed user(s) (interval: %v)", len(subscribedChats), currentInterval)
						for _, chatID := range subscribedChats {
							b.sendStatusMessages(chatID, result)
						}
						lastUpdateTime = time.Now()
					}
				}
			}
		}
	}
}

// sendTrafficChart sends the traffic chart as a photo with caption
func (b *Bot) sendTrafficChart(chatID interface{}, data *models.TrafficData) {
	if data == nil || data.ChartBuffer == nil || data.ChartBuffer.Len() == 0 {
		return
	}
	
	caption := monitor.FormatTrafficStatus(data)
	
	fileBytes := tgbotapi.FileBytes{
		Name:  "iran_traffic_24h.png",
		Bytes: data.ChartBuffer.Bytes(),
	}
	
	var photo tgbotapi.PhotoConfig
	switch id := chatID.(type) {
	case int64:
		photo = tgbotapi.NewPhoto(id, fileBytes)
	case string:
		photo = tgbotapi.NewPhotoToChannel(id, fileBytes)
	default:
		return
	}
	
	photo.Caption = caption
	photo.ParseMode = tgbotapi.ModeMarkdown
	
	_, _ = b.api.Send(photo)
}

