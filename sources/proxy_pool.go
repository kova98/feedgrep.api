package sources

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

type ProxyPool struct {
	clients     []*http.Client
	hosts       []string
	index       atomic.Uint64
	cooldowns   map[int]time.Time
	lastUsed    map[int]time.Time
	successes   map[int]int
	failures    map[int]int
	cooldownMu  sync.RWMutex
	minInterval time.Duration // minimum time between uses of the same proxy
}

func NewProxyPool(proxyURLs []string) (*ProxyPool, error) {
	clients := make([]*http.Client, 0, len(proxyURLs))
	hosts := make([]string, 0, len(proxyURLs))
	seen := make(map[string]bool)

	if len(proxyURLs) == 0 {
		return nil, errors.New("no proxy URLs provided")
	}

	for _, proxyURL := range proxyURLs {
		// Deduplicate by URL
		if seen[proxyURL] {
			if parsed, err := url.Parse(proxyURL); err == nil {
				slog.Warn("duplicate proxy URL, skipping", "host", parsed.Host)
			}
			continue
		}
		seen[proxyURL] = true

		client, err := createClient(proxyURL)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)

		// Extract host only (no credentials)
		if parsed, err := url.Parse(proxyURL); err == nil {
			hosts = append(hosts, parsed.Host)
		} else {
			hosts = append(hosts, "unknown")
		}
	}

	slog.Info("proxy pool created", "count", len(clients), "hosts", hosts)

	return &ProxyPool{
		clients:     clients,
		hosts:       hosts,
		cooldowns:   make(map[int]time.Time),
		lastUsed:    make(map[int]time.Time),
		successes:   make(map[int]int),
		failures:    make(map[int]int),
		minInterval: 30 * time.Second,
	}, nil
}

func createClient(proxyURL string) (*http.Client, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	if proxyURL == "" {
		return client, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	if parsedURL.Scheme != "socks5" {
		return client, nil
	}

	var auth *proxy.Auth
	if parsedURL.User != nil {
		password, _ := parsedURL.User.Password()
		auth = &proxy.Auth{
			User:     parsedURL.User.Username(),
			Password: password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}

	client.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	return client, nil
}

func (p *ProxyPool) Next() (*http.Client, string) {
	n := len(p.clients)

	p.cooldownMu.Lock()

	for {
		now := time.Now()

		// Try to find a proxy not on cooldown and not recently used
		for attempt := 0; attempt < n; attempt++ {
			idx := p.index.Add(1) - 1
			i := int(idx % uint64(n))

			// Skip if on rate limit cooldown
			if cooldownUntil, ok := p.cooldowns[i]; ok && now.Before(cooldownUntil) {
				continue
			}

			// Skip if used too recently
			if lastUsed, ok := p.lastUsed[i]; ok && now.Sub(lastUsed) < p.minInterval {
				continue
			}

			// Mark as used and return
			p.lastUsed[i] = now
			p.cooldownMu.Unlock()
			return p.clients[i], p.hosts[i]
		}

		// All busy/on cooldown - find the one available soonest
		var soonestAvailable time.Time
		for i := 0; i < n; i++ {
			availableAt := p.lastUsed[i].Add(p.minInterval)
			if cooldownUntil, ok := p.cooldowns[i]; ok && cooldownUntil.After(availableAt) {
				availableAt = cooldownUntil
			}

			if soonestAvailable.IsZero() || availableAt.Before(soonestAvailable) {
				soonestAvailable = availableAt
			}
		}

		// Wait until a proxy might be available, then loop back to re-check
		waitDuration := time.Until(soonestAvailable)
		if waitDuration > 0 {
			p.cooldownMu.Unlock()
			slog.Debug("all proxies busy, waiting", "wait_ms", waitDuration.Milliseconds())
			time.Sleep(waitDuration)
			p.cooldownMu.Lock()
			// Loop back to re-check - another goroutine may have taken it
		}
	}
}

// MarkRateLimited puts a proxy on cooldown for the specified duration
func (p *ProxyPool) MarkRateLimited(host string) {
	p.cooldownMu.Lock()
	defer p.cooldownMu.Unlock()

	duration := 30 * time.Second // default cooldown duration

	for i, h := range p.hosts {
		if h == host {
			p.cooldowns[i] = time.Now().Add(duration)
			slog.Debug("proxy on cooldown", "host", host, "duration_seconds", duration.Seconds())
			return
		}
	}
}

// MarkSuccess records a successful request for a proxy
func (p *ProxyPool) MarkSuccess(host string) {
	p.cooldownMu.Lock()
	defer p.cooldownMu.Unlock()

	for i, h := range p.hosts {
		if h == host {
			p.successes[i]++
			return
		}
	}
}

// MarkFailure records a failed request for a proxy
func (p *ProxyPool) MarkFailure(host string) {
	p.cooldownMu.Lock()
	defer p.cooldownMu.Unlock()

	for i, h := range p.hosts {
		if h == host {
			p.failures[i]++
			return
		}
	}
}

// GetStats returns success and failure counts for all proxies
func (p *ProxyPool) GetStats() map[string]struct{ Successes, Failures int } {
	p.cooldownMu.RLock()
	defer p.cooldownMu.RUnlock()

	stats := make(map[string]struct{ Successes, Failures int })
	for i, h := range p.hosts {
		stats[h] = struct{ Successes, Failures int }{
			Successes: p.successes[i],
			Failures:  p.failures[i],
		}
	}
	return stats
}
