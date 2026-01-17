package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
)

// RISLiveClient handles BGP monitoring via RIS Live WebSocket API
type RISLiveClient struct {
	conn          *websocket.Conn
	asnStatuses   map[string]*models.ASNStatus
	mu            sync.RWMutex
	subscribedASNs map[string]bool
	done          chan struct{}
	url           string
	reconnectMu   sync.Mutex
	reconnecting  bool
}

// RISMessage represents a message from RIS Live
type RISMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// RISUpdateMessage represents a BGP UPDATE message
type RISUpdateMessage struct {
	Timestamp   float64 `json:"timestamp"`
	Peer        string  `json:"peer"`
	PeerASN     string  `json:"peer_asn"`
	ID          string  `json:"id"`
	Host        string  `json:"host"`
	Type        string  `json:"type"`
	Path        []interface{} `json:"path,omitempty"`
	Announcements []struct {
		NextHop string   `json:"next_hop"`
		Prefixes []string `json:"prefixes"`
	} `json:"announcements,omitempty"`
	Withdrawals []string `json:"withdrawals,omitempty"`
}

// RISSubscribeMessage represents a subscription request
type RISSubscribeMessage struct {
	Type string                 `json:"type"`
	Data RISSubscribeData       `json:"data"`
}

// RISSubscribeData contains subscription parameters
type RISSubscribeData struct {
	Type         string   `json:"type"`
	PeerASN      string   `json:"peer_asn,omitempty"`
	PrefixMore   string   `json:"prefix_more,omitempty"`
	PrefixLess   string   `json:"prefix_less,omitempty"`
	PrefixExact  string   `json:"prefix_exact,omitempty"`
	Host         string   `json:"host,omitempty"`
	SocketOptions SocketOptions `json:"socketOptions"`
}

// SocketOptions for RIS Live subscription
type SocketOptions struct {
	IncludeRaw bool `json:"include_raw"`
	Acknowledge bool `json:"acknowledge"`
}

// NewRISLiveClient creates a new RIS Live client
func NewRISLiveClient(url string) (*RISLiveClient, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RIS Live: %w", err)
	}

	client := &RISLiveClient{
		conn:          conn,
		asnStatuses:   make(map[string]*models.ASNStatus),
		subscribedASNs: make(map[string]bool),
		done:          make(chan struct{}),
		url:           url,
		reconnecting:  false,
	}

	return client, nil
}

// reconnect attempts to reconnect to RIS Live WebSocket
func (c *RISLiveClient) reconnect() error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()
	
	if c.reconnecting {
		return fmt.Errorf("reconnection already in progress")
	}
	
	c.reconnecting = true
	defer func() { c.reconnecting = false }()
	
	log.Printf("Attempting to reconnect to RIS Live WebSocket...")
	
	// Close existing connection if any
	if c.conn != nil {
		c.conn.Close()
	}
	
	// Wait a bit before reconnecting
	time.Sleep(2 * time.Second)
	
	// Reconnect
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	
	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}
	
	c.conn = conn
	
	// Resubscribe to all ASNs
	c.mu.Lock()
	asns := make([]string, 0, len(c.subscribedASNs))
	for asn := range c.subscribedASNs {
		asns = append(asns, asn)
	}
	c.mu.Unlock()
	
	for _, asn := range asns {
		if err := c.SubscribeToASN(asn); err != nil {
			log.Printf("Warning: Failed to resubscribe to ASN %s after reconnect: %v", asn, err)
		}
	}
	
	log.Printf("Successfully reconnected to RIS Live WebSocket")
	return nil
}

// SubscribeToASN subscribes to BGP updates for a specific ASN
func (c *RISLiveClient) SubscribeToASN(asn string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subscribedASNs[asn] {
		return nil // Already subscribed
	}

	// Remove "AS" prefix if present
	asnNumber := asn
	if len(asn) > 2 && asn[:2] == "AS" {
		asnNumber = asn[2:]
	}

	subscribeMsg := RISSubscribeMessage{
		Type: "ris_subscribe",
		Data: RISSubscribeData{
			Type:    "UPDATE",
			PeerASN: asnNumber,
			SocketOptions: SocketOptions{
				IncludeRaw: false,
				Acknowledge: false,
			},
		},
	}

	if err := c.conn.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("failed to subscribe to ASN %s: %w", asn, err)
	}

	c.subscribedASNs[asn] = true
	
	// Initialize ASN status if not exists
	if _, exists := c.asnStatuses[asn]; !exists {
		c.asnStatuses[asn] = &models.ASNStatus{
			ASN:        asn,
			Country:    "IR",
			Name:       config.GetASNName(asn),
			Connected:  false,
			LastSeen:   time.Time{},
			LastUpdate: time.Now(),
		}
	}

	// Log subscription silently (only log errors)
	// Removed verbose subscription logging
	return nil
}

// Start starts listening for BGP messages
func (c *RISLiveClient) Start() {
	go c.readMessages()
}

// Stop stops the client
func (c *RISLiveClient) Stop() {
	close(c.done)
	if c.conn != nil {
		c.conn.Close()
	}
}

// GetASNStatuses returns current ASN statuses
func (c *RISLiveClient) GetASNStatuses() map[string]*models.ASNStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*models.ASNStatus)
	for asn, status := range c.asnStatuses {
		result[asn] = &models.ASNStatus{
			ASN:        status.ASN,
			Country:    status.Country,
			Name:       status.Name,
			Connected:  status.Connected,
			LastSeen:   status.LastSeen,
			LastUpdate: status.LastUpdate,
		}
	}
	return result
}

func (c *RISLiveClient) readMessages() {
	messageCount := 0
	lastHealthLog := time.Now()
	lastPing := time.Now()
	pingInterval := 30 * time.Second
	
	for {
		select {
		case <-c.done:
			return
		default:
			// Send ping to keep connection alive
			if time.Since(lastPing) > pingInterval {
				c.mu.RLock()
				conn := c.conn
				c.mu.RUnlock()
				
				if conn != nil {
					if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
						log.Printf("Failed to send ping: %v", err)
					} else {
						lastPing = time.Now()
					}
				}
			}
			
			// Set read deadline
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()
			
			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}
			
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			
			var msg RISMessage
			if err := conn.ReadJSON(&msg); err != nil {
				log.Printf("Error reading RIS Live message: %v", err)
				
				// Check if connection is closed or network error
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("RIS Live WebSocket connection closed, attempting to reconnect...")
					if reconnectErr := c.reconnect(); reconnectErr != nil {
						log.Printf("Reconnection failed: %v, will retry in 10 seconds", reconnectErr)
						time.Sleep(10 * time.Second)
					} else {
						// Reset counters after successful reconnect
						messageCount = 0
						lastHealthLog = time.Now()
						lastPing = time.Now()
					}
				} else {
					// Network error or timeout - try to reconnect
					log.Printf("RIS Live WebSocket error (may be transient), attempting to reconnect...")
					if reconnectErr := c.reconnect(); reconnectErr != nil {
						log.Printf("Reconnection failed: %v, will retry in 10 seconds", reconnectErr)
						time.Sleep(10 * time.Second)
					} else {
						messageCount = 0
						lastHealthLog = time.Now()
						lastPing = time.Now()
					}
				}
				continue
			}

			messageCount++
			
			// Log connection health less frequently (every 10000 messages or every 30 minutes)
			// Reduced verbosity for cleaner output
			if messageCount%10000 == 0 || time.Since(lastHealthLog) > 30*time.Minute {
				log.Printf("RIS Live connection healthy - processed %d messages", messageCount)
				lastHealthLog = time.Now()
			}

			switch msg.Type {
			case "ris_message":
				c.handleRISMessage(msg.Data)
			case "ris_error":
				var errorData struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(msg.Data, &errorData); err == nil {
					log.Printf("RIS Live error: %s", errorData.Message)
				}
			}
		}
	}
}

func (c *RISLiveClient) handleRISMessage(data json.RawMessage) {
	var update RISUpdateMessage
	if err := json.Unmarshal(data, &update); err != nil {
		log.Printf("Error unmarshaling RIS message: %v", err)
		return
	}

	if update.Type != "UPDATE" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if this update is from or about any of our monitored ASNs
	for asn := range c.subscribedASNs {
		asnNumber := asn
		if len(asn) > 2 && asn[:2] == "AS" {
			asnNumber = asn[2:]
		}

		// Check if peer ASN matches (update FROM this ASN)
		if update.PeerASN == asnNumber {
			if status, exists := c.asnStatuses[asn]; exists {
				status.Connected = true
				status.LastSeen = time.Unix(int64(update.Timestamp), 0)
				status.LastUpdate = time.Now()
			}
		}

		// Check if ASN appears in AS_PATH (update THROUGH this ASN)
		// This catches ASNs that appear in routing paths even if not as peers
		for _, pathItem := range update.Path {
			var pathASN string
			switch v := pathItem.(type) {
			case float64:
				pathASN = fmt.Sprintf("%.0f", v)
			case string:
				pathASN = v
			case []interface{}:
				// AS_SET - check all ASNs in the set
				for _, setItem := range v {
					if setASN, ok := setItem.(float64); ok {
						if fmt.Sprintf("%.0f", setASN) == asnNumber {
							if status, exists := c.asnStatuses[asn]; exists {
								status.Connected = true
								status.LastSeen = time.Unix(int64(update.Timestamp), 0)
								status.LastUpdate = time.Now()
							}
						}
					}
				}
				continue
			}

			if pathASN == asnNumber {
				if status, exists := c.asnStatuses[asn]; exists {
					status.Connected = true
					status.LastSeen = time.Unix(int64(update.Timestamp), 0)
					status.LastUpdate = time.Now()
				}
			}
		}
	}
}

// CheckConnectivity performs a connectivity check for all monitored ASNs
// Returns all subscribed ASNs, ensuring they're all included even if no updates received yet
func (c *RISLiveClient) CheckConnectivity() map[string]*models.ASNStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	result := make(map[string]*models.ASNStatus)

	// Ensure all subscribed ASNs are included in the result
	// This handles the case where statuses might not be initialized yet
	for asn := range c.subscribedASNs {
		if status, exists := c.asnStatuses[asn]; exists {
			// Consider disconnected if no update in last 30 minutes (increased from 10)
			// This is more appropriate for stable ASNs that may not send frequent updates
			timeSinceLastSeen := now.Sub(status.LastSeen)
			connected := status.Connected && timeSinceLastSeen < 30*time.Minute
			
			// Log when ASNs are marked offline for debugging
			if !connected && status.Connected {
				log.Printf("ASN %s (%s) marked offline - last seen %v ago", 
					asn, status.Name, timeSinceLastSeen)
			}
			
			result[asn] = &models.ASNStatus{
				ASN:        status.ASN,
				Country:    status.Country,
				Name:       status.Name,
				Connected:  connected,
				LastSeen:   status.LastSeen,
				LastUpdate: status.LastUpdate,
			}
		} else {
			// Initialize status if it doesn't exist (shouldn't happen, but safety check)
			result[asn] = &models.ASNStatus{
				ASN:        asn,
				Country:    "IR",
				Name:       config.GetASNName(asn),
				Connected:  false,
				LastSeen:   time.Time{},
				LastUpdate: time.Now(),
			}
		}
	}

	return result
}

