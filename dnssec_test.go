package resolver

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestDNSSECBasic(t *testing.T) {
	// Создаем сервер
	server := NewServer()
	
	// Создаем DNS запрос с DNSSEC флагом
	m := new(dns.Msg)
	m.SetQuestion("cdn.astracat.ru.", dns.TypeA)
	
	// Добавляем EDNS0 опции с DO флагом
	edns := new(dns.OPT)
	edns.Hdr.Name = "."
	edns.Hdr.Rrtype = dns.TypeOPT
	edns.Option = []dns.EDNS0{&dns.EDNS0_DO{}}
	m.Extra = append(m.Extra, edns)
	
	// Создаем mock response writer
	w := &testResponseWriter{}
	
	// Обрабатываем запрос
	server.handleQuery(w, m)
	
	if w.msg == nil {
		t.Fatal("Expected response, got nil")
	}
	
	// Проверяем наличие RRSIG записей
	var rrsigCount int
	for _, rr := range w.msg.Answer {
		if rr.Header().Rrtype == dns.TypeRRSIG {
			rrsigCount++
		}
	}
	
	if rrsigCount == 0 {
		t.Error("Expected RRSIG records in DNSSEC response")
	}
	
	// Проверяем наличие DNSKEY записей
	var dnskeyCount int
	for _, rr := range w.msg.Answer {
		if rr.Header().Rrtype == dns.TypeDNSKEY {
			dnskeyCount++
		}
	}
	
	if dnskeyCount == 0 {
		t.Error("Expected DNSKEY records in DNSSEC response")
	}
	
	// Проверяем AD флаг
	if !w.msg.AuthenticatedData {
		t.Error("Expected AuthenticatedData flag to be set")
	}
}

func TestWithoutDNSSEC(t *testing.T) {
	// Создаем сервер
	server := NewServer()
	
	// Создаем обычный DNS запрос без DNSSEC
	m := new(dns.Msg)
	m.SetQuestion("cdn.astracat.ru.", dns.TypeA)
	
	// Создаем mock response writer
	w := &testResponseWriter{}
	
	// Обрабатываем запрос
	server.handleQuery(w, m)
	
	if w.msg == nil {
		t.Fatal("Expected response, got nil")
	}
	
	// Проверяем отсутствие RRSIG записей
	for _, rr := range w.msg.Answer {
		if rr.Header().Rrtype == dns.TypeRRSIG {
			t.Error("Unexpected RRSIG record in non-DNSSEC response")
		}
	}
	
	// Проверяем отсутствие DNSKEY записей
	for _, rr := range w.msg.Answer {
		if rr.Header().Rrtype == dns.TypeDNSKEY {
			t.Error("Unexpected DNSKEY record in non-DNSSEC response")
		}
	}
	
	// Проверяем AD флаг
	if w.msg.AuthenticatedData {
		t.Error("Unexpected AuthenticatedData flag in non-DNSSEC response")
	}
}

// testResponseWriter реализует dns.ResponseWriter для тестов
type testResponseWriter struct {
	msg *dns.Msg
}

func (w *testResponseWriter) WriteMsg(m *dns.Msg) error {
	w.msg = m
	return nil
}

func (w *testResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (w *testResponseWriter) Close() error {
	return nil
}

func (w *testResponseWriter) TsigStatus() error {
	return nil
}

func (w *testResponseWriter) TsigTimersOnly(bool) {}

func (w *testResponseWriter) Hijack() {}

func (w *testResponseWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (w *testResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5300}
}