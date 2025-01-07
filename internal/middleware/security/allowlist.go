package security

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/metrics"
)

type BuildkiteMeta struct {
	WebhookIPs []string `json:"webhook_ips"`
}

type IPAllowList struct {
	mu           sync.RWMutex
	allowedIPs   map[string]struct{}
	refreshToken string
	lastUpdate   time.Time
}

func NewIPAllowList(refreshToken string) (*IPAllowList, error) {
	wl := &IPAllowList{
		allowedIPs:   make(map[string]struct{}),
		refreshToken: refreshToken,
	}

	// Initial fetch of IPs
	if err := wl.refreshIPs(); err != nil {
		return nil, err
	}

	// Start background refresh
	go wl.periodicRefresh()

	return wl, nil
}

func (wl *IPAllowList) refreshIPs() error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://api.buildkite.com/v2/meta", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if wl.refreshToken != "" {
		req.Header.Set("Authorization", "Bearer "+wl.refreshToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching meta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var meta BuildkiteMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	// Update the allowed IPs
	wl.mu.Lock()
	wl.allowedIPs = make(map[string]struct{})
	for _, ip := range meta.WebhookIPs {
		wl.allowedIPs[ip] = struct{}{}
	}
	wl.lastUpdate = time.Now()
	wl.mu.Unlock()

	return nil
}

func (wl *IPAllowList) periodicRefresh() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		if err := wl.refreshIPs(); err != nil {
			metrics.ErrorsTotal.WithLabelValues("ip_refresh").Inc()
		}
	}
}

func (wl *IPAllowList) isAllowed(ip string) bool {
	wl.mu.RLock()
	defer wl.mu.RUnlock()
	_, exists := wl.allowedIPs[ip]
	return exists
}

func (wl *IPAllowList) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get IP from X-Forwarded-For if behind a proxy
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		}

		// Handle X-Forwarded-For with multiple IPs (take the first one)
		if strings.Contains(ip, ",") {
			ip = strings.TrimSpace(strings.Split(ip, ",")[0])
		}

		// Remove port if present
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}

		if !wl.isAllowed(ip) {
			metrics.ErrorsTotal.WithLabelValues("ip_forbidden").Inc()
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
