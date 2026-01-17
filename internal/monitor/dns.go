package monitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/netblocks/netblocks/internal/config"
	"github.com/netblocks/netblocks/internal/models"
)

// DNSMonitor handles DNS server monitoring
type DNSMonitor struct {
	servers    []config.DNSServer
	statuses   map[string]*models.DNSStatus
	mu         sync.RWMutex
	timeout    time.Duration
}

// NewDNSMonitor creates a new DNS monitor
func NewDNSMonitor(servers []config.DNSServer, timeout time.Duration) *DNSMonitor {
	statuses := make(map[string]*models.DNSStatus)
	for _, server := range servers {
		// Use composite key (address:name) to handle duplicate IPs with different names
		key := server.Address + ":" + server.Name
		statuses[key] = &models.DNSStatus{
			Server:    server.Address,
			Name:      server.Name,
			Alive:     false,
			LastCheck: time.Time{},
		}
	}

	// Increase default timeout from 5s to 8s for better reliability
	if timeout < 8*time.Second {
		timeout = 8 * time.Second
	}

	return &DNSMonitor{
		servers:  servers,
		statuses: statuses,
		timeout:  timeout,
	}
}

// isNetworkError checks if an error is a network-level error (timeout, connection refused, etc.)
// These errors indicate the server is truly offline/unreachable
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	
	// Check for common network errors
	networkErrorPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"host unreachable",
		"i/o timeout",
		"connection timed out",
		"dial tcp",
		"dial udp",
	}

	for _, pattern := range networkErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Check for net.OpError and net.DNSError
	var opErr *net.OpError
	var dnsErr *net.DNSError
	if errors.As(err, &opErr) || errors.As(err, &dnsErr) {
		return true
	}

	return false
}

// CheckAll checks all DNS servers
func (dm *DNSMonitor) CheckAll(ctx context.Context) map[string]*models.DNSStatus {
	var wg sync.WaitGroup
	results := make(map[string]*models.DNSStatus)
	mu := sync.Mutex{}
	
	// Track IP addresses that are confirmed alive to prevent overwriting with failed checks
	aliveIPs := make(map[string]bool)

	for _, server := range dm.servers {
		wg.Add(1)
		go func(srv config.DNSServer) {
			defer wg.Done()
			status := dm.checkServer(ctx, srv)
			
			mu.Lock()
			// Use composite key (address:name) to handle duplicate IPs with different names
			key := srv.Address + ":" + srv.Name
			
			// If this IP was already confirmed alive by another concurrent check,
			// mark this entry as alive too (same IP, different name)
			if !status.Alive && aliveIPs[srv.Address] {
				status.Alive = true
				status.Error = "" // Clear error since IP is confirmed alive
				log.Printf("DNS server %s (%s) marked alive (IP %s confirmed alive by another check)", 
					srv.Address, srv.Name, srv.Address)
			}
			
			// Track alive IPs
			if status.Alive {
				aliveIPs[srv.Address] = true
			}
			
			results[key] = status
			mu.Unlock()
		}(server)
	}

	wg.Wait()
	
	// Ensure all statuses are updated in dm.statuses map
	// Use composite keys to preserve all entries
	dm.mu.Lock()
	for key, status := range results {
		dm.statuses[key] = status
	}
	dm.mu.Unlock()
	
	return results
}

// checkServer checks a single DNS server with retry logic for transient network errors
func (dm *DNSMonitor) checkServer(ctx context.Context, server config.DNSServer) *models.DNSStatus {
	start := time.Now()
	
	// Create DNS client
	client := &dns.Client{
		Timeout: dm.timeout,
	}

	// Create a DNS message for leader.ir
	msg := new(dns.Msg)
	msg.SetQuestion("leader.ir.", dns.TypeA)
	// Set RecursionDesired based on server type (if specified)
	// For authoritative servers, recursion may be refused, but that's OK
	// Any DNS response (even REFUSED/NOTAUTH) means the server is online
	if server.Type == "" || server.Type == "both" || server.Type == "recursive" {
		msg.RecursionDesired = true
	} else {
		// For authoritative-only servers, don't request recursion
		msg.RecursionDesired = false
	}

	// Determine if IPv4 or IPv6
	address := server.Address
	if address[0] != '[' {
		address = address + ":53"
	} else {
		address = address + ":53"
	}

	// Retry logic with exponential backoff for transient network errors
	maxRetries := 2
	baseDelay := 100 * time.Millisecond
	var r *dns.Msg
	var err error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				err = ctx.Err()
				break
			case <-time.After(delay):
				// Continue with retry
			}
		}
		
		// Query the DNS server
		r, _, err = client.Exchange(msg, address)
		
		// If we got a response (even with error code), server is alive - no retry needed
		if r != nil {
			break
		}
		
		// If it's not a network error, don't retry (e.g., DNS protocol errors)
		if err != nil && !isNetworkError(err) {
			break
		}
		
		// If context is cancelled, don't retry
		if err != nil && err == ctx.Err() {
			break
		}
		
		// For network errors, retry (transient issues like packet loss)
		if err != nil && attempt < maxRetries {
			log.Printf("DNS server %s (%s) retry attempt %d/%d: %v", 
				server.Address, server.Name, attempt+1, maxRetries, err)
		}
	}
	
	responseTime := time.Since(start)
	
	status := &models.DNSStatus{
		Server:      server.Address,
		Name:        server.Name,
		LastCheck:   time.Now(),
		ResponseTime: responseTime,
	}

	if err != nil {
		// Check if it's a network error (server truly offline) vs other error
		if isNetworkError(err) {
			status.Alive = false
			status.Error = fmt.Sprintf("Network error: %v", err)
			log.Printf("DNS server %s (%s) is offline: %v", server.Address, server.Name, err)
		} else {
			// Unexpected error, but might be transient - mark as offline but log
			status.Alive = false
			status.Error = fmt.Sprintf("Error: %v", err)
			log.Printf("DNS server %s (%s) check failed: %v", server.Address, server.Name, err)
		}
	} else if r != nil {
		// ANY DNS response means the server is alive and responding
		// Response codes like NOTAUTH, REFUSED, NXDOMAIN still mean server is online
		status.Alive = true
		
		if r.Rcode != dns.RcodeSuccess {
			// Server responded but with a non-success code - still alive!
			rcodeName := dns.RcodeToString[r.Rcode]
			status.Error = fmt.Sprintf("DNS response: %s (rcode %d)", rcodeName, r.Rcode)
			log.Printf("DNS server %s (%s) responded with %s - server is online", 
				server.Address, server.Name, rcodeName)
		}
		// If RcodeSuccess, no error message needed - server is working perfectly
	} else {
		// This shouldn't happen (err == nil but r == nil), but handle it
		status.Alive = false
		status.Error = "DNS query returned nil response"
		log.Printf("DNS server %s (%s) returned nil response", server.Address, server.Name)
	}

	// Use composite key to handle duplicate IPs with different names
	key := server.Address + ":" + server.Name
	
	dm.mu.Lock()
	// If IP is already confirmed alive, preserve that status
	if existing, exists := dm.statuses[key]; exists && existing.Alive && !status.Alive {
		// Don't overwrite alive status with dead status for the same IP
		// This handles race conditions in concurrent checks
		status = existing
	} else {
		dm.statuses[key] = status
	}
	dm.mu.Unlock()
	return status
}

// GetStatuses returns current DNS server statuses
func (dm *DNSMonitor) GetStatuses() map[string]*models.DNSStatus {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make(map[string]*models.DNSStatus)
	for addr, status := range dm.statuses {
		result[addr] = &models.DNSStatus{
			Server:      status.Server,
			Name:        status.Name,
			Alive:       status.Alive,
			ResponseTime: status.ResponseTime,
			LastCheck:   status.LastCheck,
			Error:       status.Error,
		}
	}
	return result
}

// StartPeriodicCheck starts periodic DNS checks
// Note: Initial check is performed synchronously in Monitor.Start() to ensure
// results are available before first status display
func (dm *DNSMonitor) StartPeriodicCheck(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Performing periodic DNS check...")
			dm.CheckAll(ctx)
		}
	}
}

