package resolver

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"hash/fnv"
	"sync"
	"time"
)

type Server struct {
	resolver *Resolver
	cache    *DNSCache
	workers  int
	queries  chan queryRequest
}

type queryRequest struct {
	w dns.ResponseWriter
	r *dns.Msg
}

type DNSCache struct {
	shards    [32]*cacheShard
}

type cacheShard struct {
	mu    sync.RWMutex
	items map[string]*cacheEntry
}

type cacheEntry struct {
	msg      *dns.Msg
	expires  time.Time
	dsRecord []dns.DS
}

func NewServer() *Server {
	cache := &DNSCache{}
	// Инициализируем шарды кэша
	for i := range cache.shards {
		cache.shards[i] = &cacheShard{
			items: make(map[string]*cacheEntry),
		}
	}

	s := &Server{
		resolver: NewResolver(),
		cache:    cache,
		workers:  10,
		queries:  make(chan queryRequest, 100),
	}
	
	// Запускаем воркеры для параллельной обработки
	for i := 0; i < s.workers; i++ {
		go s.worker()
	}
	
	return s
}

func (s *Server) Start() error {
	dns.HandleFunc(".", s.handleDNS)

	server := &dns.Server{
		Addr: ":5355",
		Net:  "udp",
	}

	fmt.Printf("Starting DNS server on port 5355\n")
	return server.ListenAndServe()
}

func (s *Server) worker() {
	for query := range s.queries {
		s.processQuery(query.w, query.r)
	}
}

func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	// Отправляем запрос в очередь для обработки
	s.queries <- queryRequest{w: w, r: r}
}

func (s *Server) processQuery(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = false

	// Включаем DNSSEC если запрошено
	if opt := r.IsEdns0(); opt != nil {
		m.SetEdns0(4096, opt.Do())
	}

	// Проверяем кэш перед резолвингом
	if cached := s.cache.get(r.Question[0], r.Id); cached != nil {
		w.WriteMsg(cached)
		return
	}

	// Выполняем резолвинг
	resp := s.resolver.Exchange(context.Background(), r)
	if resp.HasError() {
		m.Rcode = dns.RcodeServerFailure
		w.WriteMsg(m)
		return
	}

	// Кэшируем ответ
	s.cache.set(r.Question[0], resp.Msg)

	w.WriteMsg(resp.Msg)
}

func (c *DNSCache) getShard(key string) *cacheShard {
	h := fnv.New32()
	h.Write([]byte(key))
	return c.shards[h.Sum32()%uint32(len(c.shards))]
}

func (c *DNSCache) get(q dns.Question, requestID uint16) *dns.Msg {
	key := fmt.Sprintf("%s-%d-%d", q.Name, q.Qtype, q.Qclass)
	shard := c.getShard(key)

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if entry, exists := shard.items[key]; exists && time.Now().Before(entry.expires) {
		copy := entry.msg.Copy()
		copy.Id = requestID
		return copy
	}
	return nil
}

func (c *DNSCache) set(q dns.Question, msg *dns.Msg) {
	key := fmt.Sprintf("%s-%d-%d", q.Name, q.Qtype, q.Qclass)
	shard := c.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	ttl := uint32(3600) // Значение по умолчанию 1 час

	// Находим минимальный TTL из всех записей
	for _, rr := range msg.Answer {
		if rr.Header().Ttl < ttl {
			ttl = rr.Header().Ttl
		}
	}

	shard.items[key] = &cacheEntry{
		msg:     msg.Copy(),
		expires: time.Now().Add(time.Duration(ttl) * time.Second),
	}
}