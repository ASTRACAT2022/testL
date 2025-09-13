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
	msg := new(dns.Msg)
	msg.SetQuestion("cdn.astracat.ru.", dns.TypeA)
	
	// Добавляем EDNS0 опции с DO флагом
	edns := new(dns.OPT)
	edns.Hdr.Name = "."
	edns.Hdr.Rrtype = dns.TypeOPT
	edns.SetDo()
	edns.SetUDPSize(4096)
	msg.Extra = append(msg.Extra, edns)
	
	// Создаем тестовый response writer
	writer := &testResponseWriter{}
	
	// Обрабатываем запрос
	server.processQuery(writer, msg)
	
	// Проверяем что есть ответ
	if writer.Msg == nil {
		t.Fatal("Expected response message")
	}
	
	t.Logf("Response: %d answer records", len(writer.Msg.Answer))
	
	// Проверяем наличие записей
	for _, rr := range writer.Msg.Answer {
		t.Logf("Record: %s %s", dns.TypeToString[rr.Header().Rrtype], rr.Header().Name)
	}
}

// testResponseWriter - мок для dns.ResponseWriter
type testResponseWriter struct {
	Msg *dns.Msg
}

func (t *testResponseWriter) WriteMsg(m *dns.Msg) error {
	t.Msg = m
	return nil
}

func (t *testResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (t *testResponseWriter) Close() error {
	return nil
}

func (t *testResponseWriter) TsigStatus() error {
	return nil
}

func (t *testResponseWriter) TsigTimersOnly(bool) {}

func (t *testResponseWriter) Hijack() {}

func (t *testResponseWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (t *testResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5355}
}