package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/nsmithuk/resolver"
)

func main() {
	var (
		port     = flag.String("port", "53", "DNS server port")
		config   = flag.String("config", "optimized", "Configuration: optimized, ultrafast, balanced")
		cache  = flag.Bool("cache", true, "Enable DNS caching")
		warmup = flag.Bool("warmup", true, "Enable cache warmup")
	)
	flag.Parse()

	// Apply configuration
	switch *config {
	case "optimized":
		resolver.ApplyOptimizedConfig()
		log.Println("✅ Applied optimized configuration")
	case "ultrafast":
		resolver.ApplyUltraFastConfig()
		log.Println("⚡ Applied ultra-fast configuration")
	case "balanced":
		resolver.ApplyBalancedConfig()
		log.Println("⚖️ Applied balanced configuration")
	default:
		log.Printf("Unknown config '%s', using optimized", *config)
		resolver.ApplyOptimizedConfig()
	}

	// Create fast resolver
	var fastResolver *resolver.FastResolver
	if *cache {
		fastResolver = resolver.NewFastResolver(resolver.Cache)
		log.Println("✅ Fast resolver with cache enabled")
	} else {
		fastResolver = resolver.NewFastResolver(nil)
		log.Println("⚠️ Fast resolver without cache")
	}

	// Start cache warming
	if *warmup && *cache {
		fastResolver.StartCacheWarming()
		log.Println("🔥 Cache warming started")
	}

	// Create DNS server
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDNSRequest(w, r, fastResolver)
	})

	// Setup server
	server := &dns.Server{
		Addr:    ":" + *port,
		Net:     "udp",
		Handler: dns.DefaultServeMux,
	}

	// Start server in background
	go func() {
		log.Printf("🚀 Starting optimized DNS server on port %s...", *port)
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 Shutting down DNS server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.ShutdownContext(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("✅ DNS server stopped")
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg, fastResolver *resolver.FastResolver) {
	if len(r.Question) == 0 {
		return
	}

	question := r.Question[0]
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())

	// Log request
	log.Printf("📥 Query from %s: %s %s", clientIP, question.Name, dns.TypeToString[question.Qtype])

	// Use fast resolver
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	response, err := fastResolver.ExchangeWithOptimization(ctx, question)
	if err != nil {
		log.Printf("❌ Error resolving %s: %v", question.Name, err)
		
		// Return SERVFAIL
		m := new(dns.Msg)
		m.SetReply(r)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return
	}

	if response == nil {
		// Return empty response
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
		return
	}

	// Log response
	rcode := dns.RcodeToString[response.Rcode]
	log.Printf("📤 Response for %s: %s (%d answers)", question.Name, rcode, len(response.Answer))

	// Send response
	w.WriteMsg(response)
}

// Build and run:
// go build -o optimized_dns optimized_server.go
// ./optimized_dns -port=5353 -config=optimized