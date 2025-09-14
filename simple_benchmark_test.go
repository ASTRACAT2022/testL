// +build performance

package resolver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func BenchmarkResolverPerformance(b *testing.B) {
	// Создаем простой резолвер
	r := &Resolver{}
	
	ctx := context.Background()
	msg := &dns.Msg{}
	msg.SetQuestion("google.com.", dns.TypeA)
	msg.RecursionDesired = true
	
	b.ResetTimer()
	b.Run("StandardResolver", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = r.Exchange(ctx, msg)
		}
	})
}

func TestQuickPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}
	
	r := &Resolver{}
	ctx := context.Background()
	msg := &dns.Msg{}
	msg.SetQuestion("google.com.", dns.TypeA)
	msg.RecursionDesired = true
	
	// Запускаем 100 запросов для тестирования
	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = r.Exchange(ctx, msg)
	}
	duration := time.Since(start)
	
	fmt.Printf("100 DNS queries completed in %v\n", duration)
	fmt.Printf("Average per query: %v\n", duration/100)
	fmt.Printf("QPS: %.2f\n", 100.0/duration.Seconds())
}

func TestConcurrentPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}
	
	r := &Resolver{}
	ctx := context.Background()
	
	concurrent := 10
	queriesPerWorker := 5
	
	start := time.Now()
	
	for i := 0; i < concurrent; i++ {
		msg := &dns.Msg{}
		msg.SetQuestion(fmt.Sprintf("test%d.com.", i), dns.TypeA)
		msg.RecursionDesired = true
		
		for j := 0; j < queriesPerWorker; j++ {
			_ = r.Exchange(ctx, msg)
		}
	}
	
	duration := time.Since(start)
	totalQueries := concurrent * queriesPerWorker
	
	fmt.Printf("Concurrent test: %d queries in %v\n", totalQueries, duration)
	fmt.Printf("QPS: %.2f\n", float64(totalQueries)/duration.Seconds())
}