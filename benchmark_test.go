package resolver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func BenchmarkStandardResolver(b *testing.B) {
	resolver := &Resolver{}
	ctx := context.Background()

	msg := &dns.Msg{}
	msg.SetQuestion("google.com.", dns.TypeA)
	msg.RecursionDesired = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resolver.Exchange(ctx, msg)
	}
}

func BenchmarkFastResolver(b *testing.B) {
	cache := Cache
	resolver := NewFastResolver(cache)
	ctx := context.Background()

	question := dns.Question{
		Name:   "google.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolver.ExchangeWithOptimization(ctx, question)
	}
}

func BenchmarkOptimizedConfig(b *testing.B) {
	ApplyOptimizedConfig()
	resolver := &Resolver{}
	ctx := context.Background()

	msg := &dns.Msg{}
	msg.SetQuestion("google.com.", dns.TypeA)
	msg.RecursionDesired = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolver.Exchange(ctx, msg)
	}
}

func BenchmarkUltraFastConfig(b *testing.B) {
	ApplyUltraFastConfig()
	resolver := &Resolver{}
	ctx := context.Background()

	msg := &dns.Msg{}
	msg.SetQuestion("google.com.", dns.TypeA)
	msg.RecursionDesired = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolver.Exchange(ctx, msg)
	}
}

func BenchmarkConcurrentQueries(b *testing.B) {
	cache := Cache
	resolver := NewFastResolver(cache)
	ctx := context.Background()

	domains := []string{
		"google.com.",
		"facebook.com.",
		"youtube.com.",
		"twitter.com.",
		"instagram.com.",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			domain := domains[i%len(domains)]
			question := dns.Question{
				Name:   domain,
				Qtype:  dns.TypeA,
				Qclass: dns.ClassINET,
			}
			_, _ = resolver.ExchangeWithOptimization(ctx, question)
			i++
		}
	})
}

func BenchmarkCachePerformance(b *testing.B) {
	cache := Cache
	resolver := NewFastResolver(cache)
	ctx := context.Background()

	// Warm cache
	for i := 0; i < 100; i++ {
		question := dns.Question{
			Name:   fmt.Sprintf("test%d.com.", i),
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
		_, _ = resolver.ExchangeWithOptimization(ctx, question)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		question := dns.Question{
			Name:   fmt.Sprintf("test%d.com.", i%100),
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
		_, _ = resolver.ExchangeWithOptimization(ctx, question)
	}
}

func TestPerformanceComparison(t *testing.T) {
	// Test different configurations
	testCases := []struct {
		name string
		fn   func(context.Context, dns.Question) (*dns.Msg, error)
	}{
		{"Standard", func(ctx context.Context, q dns.Question) (*dns.Msg, error) {
			resolver := &Resolver{}
			msg := &dns.Msg{}
			msg.SetQuestion(q.Name, q.Qtype)
			msg.RecursionDesired = true
			return resolver.Exchange(ctx, msg)
		}},
		{"Fast", func(ctx context.Context, q dns.Question) (*dns.Msg, error) {
			resolver := NewFastResolver(Cache)
			return resolver.ExchangeWithOptimization(ctx, q)
		}},
		{"Optimized", func(ctx context.Context, q dns.Question) (*dns.Msg, error) {
			ApplyOptimizedConfig()
			resolver := NewFastResolver(Cache)
			return resolver.ExchangeWithOptimization(ctx, q)
		}},
	}

	domain := "google.com."
	question := dns.Question{
		Name:   domain,
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			_, err := tc.fn(ctx, question)
			duration := time.Since(start)

			if err != nil {
				t.Logf("%s: error %v", tc.name, err)
			} else {
				t.Logf("%s: %v", tc.name, duration)
			}
		})
	}
}

func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cache := Cache
	resolver := NewFastResolver(cache)
	ctx := context.Background()

	var wg sync.WaitGroup
	concurrent := 50
	queries := 100

	start := time.Now()

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < queries; j++ {
				question := dns.Question{
					Name:   fmt.Sprintf("test%d-%d.com.", worker, j),
					Qtype:  dns.TypeA,
					Qclass: dns.ClassINET,
				}
				_, _ = resolver.ExchangeWithOptimization(ctx, question)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Stress test completed: %d concurrent, %d queries each, total %d queries in %v",
		concurrent, queries, concurrent*queries, duration)
	t.Logf("QPS: %.2f", float64(concurrent*queries)/duration.Seconds())
}
