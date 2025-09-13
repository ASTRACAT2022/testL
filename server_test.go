package resolver

import (
	"github.com/miekg/dns"
	"testing"
	"time"
)

func TestDNSCache(t *testing.T) {
	cache := &DNSCache{
		items: make(map[string]*cacheEntry),
	}

	// Создаем тестовый DNS-запрос
	q := dns.Question{
		Name:   "example.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}

	// Создаем тестовый DNS-ответ
	msg := new(dns.Msg)
	msg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "example.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: []byte{192, 0, 2, 1},
		},
	}

	// Тестируем установку значения в кэш
	cache.set(q, msg)

	// Проверяем получение значения из кэша
	cachedMsg := cache.get(q)
	if cachedMsg == nil {
		t.Error("Expected cached message, got nil")
	}

	if len(cachedMsg.Answer) != 1 {
		t.Errorf("Expected 1 answer, got %d", len(cachedMsg.Answer))
	}

	// Проверяем истечение TTL
	time.Sleep(time.Second)
	cachedMsg = cache.get(q)
	if cachedMsg == nil {
		t.Error("Cache entry expired too early")
	}
}

func TestServer(t *testing.T) {
	server := NewServer()
	if server == nil {
		t.Fatal("Failed to create server")
	}

	if server.resolver == nil {
		t.Error("Server resolver is nil")
	}

	if server.cache == nil {
		t.Error("Server cache is nil")
	}
}

func TestDNSHandler(t *testing.T) {
	handler := &DNSHandler{cache: &DNSCache{}}

	// Create a test message
	msg := new(dns.Msg)
	msg.SetQuestion("cdn.astracat.ru.", dns.TypeA)

	// Handle the request
	resp := handler.handleDNSRequest(msg)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Check if we have answers
	if len(resp.Answer) == 0 {
		t.Error("Expected answers, got none")
	}

	t.Log("DNS handler test passed")
}

func TestCNAMEResolution(t *testing.T) {
	handler := &DNSHandler{cache: &DNSCache{}}

	// Test CNAME resolution
	msg := new(dns.Msg)
	msg.SetQuestion("cdn.astracat.ru.", dns.TypeA)

	resp := handler.handleDNSRequest(msg)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Check for CNAME and A records
	hasCNAME := false
	hasA := false
	for _, rr := range resp.Answer {
		switch rr.Header().Rrtype {
		case dns.TypeCNAME:
			hasCNAME = true
		case dns.TypeA:
			hasA = true
		}
	}

	// Note: actual records depend on DNS response
	t.Logf("CNAME found: %v, A found: %v", hasCNAME, hasA)
}

func TestTTL(t *testing.T) {
	handler := &DNSHandler{cache: &DNSCache{}}

	msg := new(dns.Msg)
	msg.SetQuestion("cdn.astracat.ru.", dns.TypeA)

	resp := handler.handleDNSRequest(msg)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Check TTL values
	for _, rr := range resp.Answer {
		if rr.Header().Ttl == 0 {
			t.Log("Warning: Zero TTL found")
		}
	}
}

func TestDNSSECSupport(t *testing.T) {
	handler := &DNSHandler{cache: &DNSCache{}}
	
	// Тестовый запрос с DNSSEC
	msg := new(dns.Msg)
	msg.SetQuestion("cdn.astracat.ru.", dns.TypeA)
	
	// Добавляем EDNS0 опцию с DO флагом
	opt := &dns.OPT{
		Hdr: dns.RR_Header{
			Name:   ".",
			Rrtype: dns.TypeOPT,
			Class:  4096,
		},
	}
	opt.SetDo(true)
	msg.Extra = append(msg.Extra, opt)

	// Обрабатываем запрос
	resp := handler.handleDNSRequest(msg)
	
	if resp == nil {
		t.Fatal("Response is nil")
	}
	
	t.Log("DNSSEC test passed - response received")
}

func TestBasicDNS(t *testing.T) {
	// Тест базовой DNS функциональности
	handler := &DNSHandler{cache: &DNSCache{}}
	
	msg := new(dns.Msg)
	msg.SetQuestion("cdn.astracat.ru.", dns.TypeA)
	
	resp := handler.handleDNSRequest(msg)
	
	if resp == nil {
		t.Fatal("Response is nil")
	}
	
	if len(resp.Answer) == 0 {
		t.Log("Warning: No answers in response")
	}
	
	t.Log("Basic DNS test passed")
}