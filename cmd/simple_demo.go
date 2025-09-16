// +build simple

//go:build simple
// +build simple

//go:build simple

//go:build simple

//go:build simple

//go:build simple

//go:build simple

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/miekg/dns"
	"github.com/nsmithuk/resolver"
)

func main() {
	fmt.Println("🚀 DNS Resolver Performance Test")
	fmt.Println("================================")

	// Создаем стандартный резолвер
	standard := resolver.NewResolver()
	
	// Тестовые домены
	domains := []string{
		"google.com.",
		"github.com.",
		"stackoverflow.com.",
	}
	
	fmt.Printf("\n📊 Testing %d domains with standard resolver\n", len(domains))
	
	totalStart := time.Now()
	for _, domain := range domains {
		msg := &dns.Msg{}
		msg.SetQuestion(domain, dns.TypeA)
		msg.RecursionDesired = true
		
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp := standard.Exchange(ctx, msg)
		duration := time.Since(start)
		cancel()
		
		if resp.HasError() {
			fmt.Printf("  ❌ %s: %v\n", domain, resp.Err)
		} else {
			ipCount := 0
			if resp.Msg != nil {
				ipCount = len(resp.Msg.Answer)
			}
			fmt.Printf("  ✅ %s: %d IPs in %v\n", domain, ipCount, duration)
		}
	}
	totalTime := time.Since(totalStart)
	
	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   Total time: %v\n", totalTime)
	fmt.Printf("   Average per domain: %v\n", totalTime/time.Duration(len(domains)))
	fmt.Printf("   Domains per second: %.2f\n", float64(len(domains))/totalTime.Seconds())
	
	fmt.Println("\n✨ Test completed!")
}