package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/nsmithuk/resolver"
)

func main() {
	fmt.Println("🚀 Concurrent DNS Performance Test")
	fmt.Println("=================================")

	// Создаем резолвер
	r := resolver.NewResolver()
	
	// Тестовые домены
	domains := []string{
		"google.com.",
		"github.com.",
		"stackoverflow.com.",
		"cloudflare.com.",
		"microsoft.com.",
		"amazon.com.",
		"apple.com.",
		"netflix.com.",
	}
	
	fmt.Printf("\n📊 Testing %d domains with different concurrency levels\n", len(domains))
	
	// Тест 1: Последовательные запросы
	fmt.Println("\n1️⃣ Sequential requests:")
	start := time.Now()
	for _, domain := range domains {
		msg := &dns.Msg{}
		msg.SetQuestion(domain, dns.TypeA)
		msg.RecursionDesired = true
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp := r.Exchange(ctx, msg)
		cancel()
		
		if !resp.HasError() {
			fmt.Printf("  ✅ %s resolved\n", domain)
		}
	}
	sequentialTime := time.Since(start)
	
	// Тест 2: Конкурентные запросы
	fmt.Println("\n2️⃣ Concurrent requests (8 workers):")
	start = time.Now()
	
	var wg sync.WaitGroup
	results := make(chan string, len(domains))
	
	// Запускаем 8 конкурентных запросов
	concurrency := 8
	sem := make(chan struct{}, concurrency)
	
	for _, domain := range domains {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			sem <- struct{}{} // Захватываем слот
			defer func() { <-sem }() // Освобождаем слот
			
			msg := &dns.Msg{}
			msg.SetQuestion(d, dns.TypeA)
			msg.RecursionDesired = true
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			resp := r.Exchange(ctx, msg)
			cancel()
			
			if !resp.HasError() {
				results <- fmt.Sprintf("  ✅ %s resolved concurrently", d)
			}
		}(domain)
	}
	
	// Закрываем канал после завершения всех горутин
	go func() {
		wg.Wait()
		close(results)
	}()
	
	// Выводим результаты
	for result := range results {
		fmt.Println(result)
	}
	
	concurrentTime := time.Since(start)
	
	// Результаты
	fmt.Printf("\n📈 Performance Comparison:\n")
	fmt.Printf("   Sequential:  %v\n", sequentialTime)
	fmt.Printf("   Concurrent:  %v\n", concurrentTime)
	fmt.Printf("   Speedup:     %.1fx faster\n", float64(sequentialTime)/float64(concurrentTime))
	fmt.Printf("   Throughput:  %.2f domains/sec\n", float64(len(domains))/concurrentTime.Seconds())
	
	fmt.Println("\n✨ Concurrent test completed!")
}