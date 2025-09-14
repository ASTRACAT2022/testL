package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// FastNameserverPool - optimized pool with concurrent queries
type FastNameserverPool struct {
	fastTimeout time.Duration
	stats       *PoolStats
	mu          sync.RWMutex
}

type PoolStats struct {
	IPv4Requests uint64
	IPv6Requests uint64
	Timeouts     uint64
	Fallbacks    uint64
	sync.RWMutex
}

func NewFastNameserverPool() *FastNameserverPool {
	return &FastNameserverPool{
		fastTimeout: 30 * time.Millisecond,
		stats:       &PoolStats{},
	}
}

func (fp *FastNameserverPool) exchangeFast(ctx context.Context, servers []string, m *dns.Msg) (*dns.Msg, error) {
	if len(servers) == 0 {
		return nil, fmt.Errorf("no nameservers available")
	}

	return fp.concurrentExchange(ctx, servers, m)
}

func (fp *FastNameserverPool) concurrentExchange(ctx context.Context, servers []string, m *dns.Msg) (*dns.Msg, error) {
	resultChan := make(chan *dns.Msg, len(servers))
	var wg sync.WaitGroup

	fastCtx, cancel := context.WithTimeout(ctx, fp.fastTimeout)
	defer cancel()

	for _, server := range servers {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()

			resp := fp.quickQuery(fastCtx, s, m)
			if resp != nil {
				select {
				case resultChan <- resp:
				case <-fastCtx.Done():
				}
			}
		}(server)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	select {
	case resp := <-resultChan:
		return resp, nil
	case <-fastCtx.Done():
		return nil, context.DeadlineExceeded
	}
}

func (fp *FastNameserverPool) quickQuery(ctx context.Context, server string, m *dns.Msg) *dns.Msg {
	client := &dns.Client{
		Net:     "udp",
		Timeout: 25 * time.Millisecond,
		UDPSize: 512,
	}

	msg, _, err := client.ExchangeContext(ctx, m, server+":53")
		if err != nil {
			return nil
		}
		return msg
}

func (fp *FastNameserverPool) exchangeWithFallback(ctx context.Context, servers []string, m *dns.Msg) (*dns.Msg, error) {
	if resp, err := fp.exchangeFast(ctx, servers, m); err == nil && resp != nil {
		return resp, nil
	}

	if len(servers) > 0 {
		client := &dns.Client{
			Net:     "udp",
			Timeout: 200 * time.Millisecond,
			UDPSize: 512,
		}
		resp, _, err := client.ExchangeContext(ctx, m, servers[0]+":53")
		return resp, err
	}
	return nil, fmt.Errorf("no nameservers available")
}

func (fp *FastNameserverPool) measureLatency(servers []string) map[string]time.Duration {
	latency := make(map[string]time.Duration)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, server := range servers {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()

			start := time.Now()
			client := &dns.Client{
				Net:     "udp",
				Timeout: 100 * time.Millisecond,
			}

			msg := &dns.Msg{}
			msg.SetQuestion(".", dns.TypeNS)

			_, _, err := client.Exchange(msg, s+":53")
			if err == nil {
				mu.Lock()
				latency[s] = time.Since(start)
				mu.Unlock()
			}
		}(server)
	}

	wg.Wait()
	return latency
}

func (fp *FastNameserverPool) selectBestServers(servers []string, max int) []string {
	latency := fp.measureLatency(servers)

	var sorted []string
	for _, s := range servers {
		if lat, ok := latency[s]; ok && lat < 50*time.Millisecond {
			sorted = append(sorted, s)
		}
	}

	if len(sorted) > max {
		sorted = sorted[:max]
	}
	return sorted
}