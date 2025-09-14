package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type FastResolver struct {
	cache      CacheInterface
	prefetchCh chan string
	stats      *FastResolverStats
}

type FastResolverStats struct {
	Hits      uint64
	Misses    uint64
	Prefetch  uint64
	Timeouts  uint64
	CacheSize uint64
	sync.RWMutex
}

type ResolveResult struct {
	Response *dns.Msg
	Server   string
	Duration time.Duration
	Error    error
}

func NewFastResolver(cache CacheInterface) *FastResolver {
	fr := &FastResolver{
		cache:      cache,
		prefetchCh: make(chan string, 1000),
		stats:      &FastResolverStats{},
	}
	
	// Start prefetch worker
	go fr.prefetchWorker()
	
	return fr
}

func (r *FastResolver) ExchangeWithOptimization(ctx context.Context, question dns.Question) (*dns.Msg, error) {
	// Fast cache check
	if cached, _ := r.cache.Get("", question); cached != nil {
		r.stats.Lock()
		r.stats.Hits++
		r.stats.Unlock()
		return cached, nil
	}
	
	r.stats.Lock()
	r.stats.Misses++
	r.stats.Unlock()
	
	// Concurrent resolution
	result := r.concurrentResolve(ctx, question)
	
	// Cache successful responses
		if result.Error == nil && result.Response != nil {
			r.cache.Update("", question, result.Response)
			
			// Trigger prefetch for popular domains
			if r.shouldPrefetch(question.Name) {
				select {
				case r.prefetchCh <- question.Name:
				default:
					// Channel full, skip prefetch
				}
			}
		}
	
	return result.Response, result.Error
}

func (r *FastResolver) concurrentResolve(ctx context.Context, question dns.Question) *ResolveResult {
	servers := r.getNameservers(question.Name)
	if len(servers) == 0 {
		return &ResolveResult{Error: fmt.Errorf("no nameservers available")}
	}
	
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	
	resultChan := make(chan *ResolveResult, len(servers))
	startTime := time.Now()
	
	// Launch concurrent queries
	for _, server := range servers {
		go func(s string) {
			resp, err := r.exchangeWithFastTimeout(ctx, question, s)
			resultChan <- &ResolveResult{
				Response: resp,
				Server:   s,
				Duration: time.Since(startTime),
				Error:    err,
			}
		}(server)
	}
	
	// Wait for first successful response
	for i := 0; i < len(servers); i++ {
		select {
		case result := <-resultChan:
			if result.Error == nil && result.Response != nil && result.Response.Rcode == dns.RcodeSuccess {
				return result
			}
		case <-ctx.Done():
			return &ResolveResult{Error: ctx.Err()}
		}
	}
	
	return &ResolveResult{Error: fmt.Errorf("no nameservers available")}
}

func (r *FastResolver) exchangeWithFastTimeout(ctx context.Context, question dns.Question, server string) (*dns.Msg, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(question.Name, question.Qtype)
	msg.RecursionDesired = true
	
	client := &dns.Client{
		Net:     "udp",
		Timeout: 50 * time.Millisecond,
		UDPSize: 4096,
	}
	
	resp, _, err := client.ExchangeContext(ctx, msg, server)
	return resp, err
}

func (r *FastResolver) getNameservers(domain string) []string {
	// Simplified nameserver selection - use root hints
	return []string{
		"8.8.8.8:53",
		"8.8.4.4:53",
		"1.1.1.1:53",
		"1.0.0.1:53",
	}
}

func (r *FastResolver) shouldPrefetch(domain string) bool {
	// Simple heuristic: popular TLDs
	popularTLDs := map[string]bool{
		"com.": true,
		"net.": true,
		"org.": true,
		"io.":  true,
	}
	
	for tld := range popularTLDs {
		if len(domain) > len(tld) && domain[len(domain)-len(tld):] == tld {
			return true
		}
	}
	return false
}

func (r *FastResolver) prefetchWorker() {
	for domain := range r.prefetchCh {
		question := dns.Question{
			Name:   domain,
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		_, _ = r.ExchangeWithOptimization(ctx, question)
		cancel()
		
		r.stats.Lock()
		r.stats.Prefetch++
		r.stats.Unlock()
	}
}

func (r *FastResolver) GetStats() FastResolverStats {
	r.stats.RLock()
	defer r.stats.RUnlock()
	return *r.stats
}

func (r *FastResolver) StartCacheWarming() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			r.warmPopularDomains()
		}
	}()
}

func (r *FastResolver) warmPopularDomains() {
	popularDomains := []string{
		"google.com.",
		"facebook.com.",
		"youtube.com.",
		"twitter.com.",
		"instagram.com.",
	}
	
	for _, domain := range popularDomains {
		question := dns.Question{
			Name:   domain,
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, _ = r.ExchangeWithOptimization(ctx, question)
		cancel()
	}
}