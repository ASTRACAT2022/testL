package resolver

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"github.com/nsmithuk/resolver/dnssec"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"container/list"
)

type Server struct {
	resolver        *Resolver
	cache           *DNSCache
	workers         int
	queries         chan queryRequest
	prefetch        *prefetchManager
	dnssecValidator *dnssec.Authenticator
}

type queryRequest struct {
	w dns.ResponseWriter
	r *dns.Msg
}

type DNSCache struct {
	shards    [32]*cacheShard
	maxSize   int
	stats     CacheStats
}

type CacheStats struct {
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Expired     uint64
	Negative    uint64
}

type cacheShard struct {
	mu       sync.RWMutex
	items    map[string]*cacheEntry
	lruList  *list.List
	lruMap   map[string]*list.Element
}

type cacheEntry struct {
	msg       *dns.Msg
	expires   time.Time
	dsRecord  []dns.DS
	key       string
	isNegative bool
	frequency uint32 // для LFU-like eviction
}

func NewServer() *Server {
	cache := &DNSCache{
		maxSize: 10000, // максимальный размер кэша
	}
	// Инициализируем шарды кэша
	for i := range cache.shards {
		cache.shards[i] = &cacheShard{
			items:   make(map[string]*cacheEntry),
			lruList: list.New(),
			lruMap:  make(map[string]*list.Element),
		}
	}

	s := &Server{
		resolver:        NewResolver(cache),
		cache:         cache,
		workers:       10,
		queries:       make(chan queryRequest, 100),
		prefetch:      newPrefetchManager(cache, NewResolver(cache)),
		dnssecValidator: nil, // DNSSEC валидатор не инициализирован по умолчанию
	}
	
	// Запускаем воркеры для параллельной обработки
	for i := 0; i < s.workers; i++ {
		go s.worker()
	}
	
	// Запускаем периодическую очистку кэша
	go s.cacheCleaner()
	
	return s
}

func NewServerWithConfig(config *Config) *Server {
	cacheSize := 100000 // Увеличиваем кэш в 10 раз для всех доменов
	if config.CacheSize > 0 {
		cacheSize = config.CacheSize
	}
	
	cache := &DNSCache{
		maxSize: cacheSize,
	}
	for i := range cache.shards {
		cache.shards[i] = &cacheShard{
			items:   make(map[string]*cacheEntry),
			lruList: list.New(),
			lruMap:  make(map[string]*list.Element),
		}
	}

	s := &Server{
		resolver:        NewResolver(cache),
		cache:           cache,
		workers:       50,    // Увеличиваем количество воркеров
		queries:       make(chan queryRequest, 1000), // Увеличиваем буфер
		prefetch:      newPrefetchManager(cache, NewResolver(cache)),
		dnssecValidator: nil,
	}
	
	if config.EnableDNSSEC {
		s.dnssecValidator = dnssec.NewAuth(context.Background(), dns.Question{})
	}
	
	for i := 0; i < s.workers; i++ {
		go s.worker()
	}
	go s.cacheCleaner()
	
	return s
}

type prefetchManager struct {
	cache     *DNSCache
	resolver  *Resolver
	popular   map[string]int
	mu        sync.RWMutex
	threshold int
}

func newPrefetchManager(cache *DNSCache, resolver *Resolver) *prefetchManager {
	pm := &prefetchManager{
		cache:     cache,
		resolver:  resolver,
		popular:   make(map[string]int),
		threshold: 3, // уменьшаем порог для более агрессивного prefetch
	}
	
	// Запускаем периодическое обновление для всех доменов
	go pm.prefetchAllDomains()
	
	return pm
}

func (pm *prefetchManager) recordAccess(q dns.Question) {
	key := fmt.Sprintf("%s-%d-%d", q.Name, q.Qtype, q.Qclass)
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.popular[key]++
	
	// Если достигли порога популярности, запускаем prefetch
	if pm.popular[key] >= pm.threshold {
		go pm.prefetchRecord(q)
		// Сбрасываем счетчик после prefetch
		pm.popular[key] = 0
	}
}

func (pm *prefetchManager) prefetchRecord(q dns.Question) {
	// Проверяем, есть ли уже в кэше
	if pm.cache.get(q, 0) != nil {
		return
	}
	
	// Выполняем prefetch
	msg := new(dns.Msg)
	msg.SetQuestion(q.Name, q.Qtype)
	msg.SetEdns0(4096, true)
	
	ctx := context.Background()
	resp := pm.resolver.Exchange(ctx, msg)
	
	if !resp.HasError() {
		pm.cache.set(q, resp.Msg)
	}
}

func (pm *prefetchManager) prefetchAllDomains() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		pm.mu.Lock()
		
		// Агрессивное prefetch для всех доменов из кэша
		allDomains := pm.cache.getAllDomains()
		for _, q := range allDomains {
			go pm.prefetchRecord(q)
		}
		
		// Очищаем статистику
		pm.popular = make(map[string]int)
		pm.mu.Unlock()
	}
}

func (s *Server) Start() error {
	dns.HandleFunc(".", s.handleDNS)

	server := &dns.Server{
		Addr: ":5355",
		Net:  "udp",
	}

	fmt.Printf("Starting DNS server on port 5355\n")
	
	// Выводим статистику кэша каждую минуту
	go s.printStats()
	
	return server.ListenAndServe()
}

func (s *Server) printStats() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		stats := s.cache.Stats()
		size := s.cache.Size()
		
		total := stats.Hits + stats.Misses
		hitRate := float64(0)
		if total > 0 {
			hitRate = float64(stats.Hits) / float64(total) * 100
		}
		
		fmt.Printf("Cache stats: size=%d, hits=%d, misses=%d, hit_rate=%.2f%%, evictions=%d, expired=%d, negative=%d\n",
			size, stats.Hits, stats.Misses, hitRate, stats.Evictions, stats.Expired, stats.Negative)
	}
}

// validateDNSSEC выполняет DNSSEC валидацию ответа
func (s *Server) validateDNSSEC(ctx context.Context, msg *dns.Msg) (string, error) {
	if s.dnssecValidator == nil {
		return "Insecure", nil
	}

	// Создаем зону для валидации
	// Для упрощения используем базовую валидацию
	zone := &basicZone{name: "."}
	
	// Выполняем валидацию
	err := s.dnssecValidator.AddResponse(zone, msg)
	if err != nil {
		return "Bogus", err
	}

	// Получаем результат валидации
	result, _, _ := s.dnssecValidator.Result()
	if result == dnssec.Unknown {
		return "Insecure", nil
	}

	return result.String(), nil
}

// basicZone - простая реализация интерфейса Zone для DNSSEC
type basicZone struct {
	name string
}

func (z *basicZone) Name() string {
	return z.name
}

func (z *basicZone) GetDNSKEYRecords() ([]dns.RR, error) {
	// Возвращаем базовые DNSKEY записи
	return []dns.RR{}, nil
}

func (z *basicZone) GetDSRecords() ([]dns.RR, error) {
	// Возвращаем базовые DS записи
	return []dns.RR{}, nil
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
	opt := r.IsEdns0()
	if opt != nil {
		m.SetEdns0(4096, opt.Do())
	}

	// Проверяем кэш перед резолвингом
	if cached := s.cache.get(r.Question[0], r.Id); cached != nil {
		// Добавляем DNSSEC флаг к кэшированному ответу если включено
		if opt != nil && opt.Do() {
			cached.AuthenticatedData = true
		}
		w.WriteMsg(cached)
		return
	}
	
	// Регистрируем обращение для prefetch анализа
	s.prefetch.recordAccess(r.Question[0])

	// Выполняем резолвинг с DNSSEC валидацией
	ctx := context.Background()
	
	// Выполняем резолвинг
	resp := s.resolver.Exchange(ctx, r)
	if resp.HasError() {
		// Negative caching для NXDOMAIN и других ошибок
		if resp.Err.Error() == "NXDOMAIN" {
			s.cache.setNegative(r.Question[0], dns.RcodeNameError)
			m.Rcode = dns.RcodeNameError
		} else {
			s.cache.setNegative(r.Question[0], dns.RcodeServerFailure)
			m.Rcode = dns.RcodeServerFailure
		}
		w.WriteMsg(m)
		return
	}

	// DNSSEC валидация если включено
	if opt != nil && opt.Do() {
		// Проверяем DNSSEC валидацию
		if s.dnssecValidator != nil {
			authResult, err := s.validateDNSSEC(ctx, resp.Msg)
			if err != nil || authResult != "Secure" {
				// Ошибка валидации DNSSEC - возвращаем SERVFAIL
				m.Rcode = dns.RcodeServerFailure
				w.WriteMsg(m)
				return
			}
			
			// Валидация прошла успешно
			resp.Msg.AuthenticatedData = true
		} else {
			// DNSSEC включен но валидатор не настроен
			resp.Msg.AuthenticatedData = false
		}
	}

	// Кэшируем ответ
	s.cache.set(r.Question[0], resp.Msg)

	w.WriteMsg(resp.Msg)
}

func (c *DNSCache) getShard(key string) *cacheShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()%uint32(len(c.shards))]
}

func (c *DNSCache) get(q dns.Question, requestID uint16) *dns.Msg {
	key := fmt.Sprintf("%s-%d-%d", q.Name, q.Qtype, q.Qclass)
	shard := c.getShard(key)

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if elem, exists := shard.lruMap[key]; exists {
		entry := elem.Value.(*cacheEntry)
		
		if time.Now().Before(entry.expires) {
			atomic.AddUint64(&c.stats.Hits, 1)
			entry.frequency++
			shard.lruList.MoveToFront(elem)
			
			copy := entry.msg.Copy()
			copy.Id = requestID
			return copy
		} else {
			atomic.AddUint64(&c.stats.Expired, 1)
			c.evictEntry(shard, elem)
		}
	}
	
	atomic.AddUint64(&c.stats.Misses, 1)
	return nil
}

func (c *DNSCache) set(q dns.Question, msg *dns.Msg) {
	key := fmt.Sprintf("%s-%d-%d", q.Name, q.Qtype, q.Qclass)
	shard := c.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Проверяем размер кэша и удаляем старые записи при необходимости
	c.ensureCapacity(shard)

	ttl := uint32(3600) // Значение по умолчанию 1 час

	// Находим минимальный TTL из всех записей
	for _, rr := range msg.Answer {
		if rr.Header().Ttl < ttl {
			ttl = rr.Header().Ttl
		}
	}
	
	for _, rr := range msg.Ns {
		if rr.Header().Ttl < ttl {
			ttl = rr.Header().Ttl
		}
	}
	
	for _, rr := range msg.Extra {
		if rr.Header().Ttl < ttl {
			ttl = rr.Header().Ttl
		}
	}
	
	// Если TTL = 0, используем минимальный разумный
	if ttl == 0 {
		ttl = 300 // 5 минут
	}
	
	// Увеличиваем максимальный TTL для максимального кэширования
		if ttl > 86400 {
			ttl = 86400 // max 24 hours
		}
		
		// Минимальный TTL для предотвращения слишком частого обновления
		if ttl < 60 {
			ttl = 60 // min 1 minute
		}
		
		expires := time.Now().Add(time.Duration(ttl) * time.Second)
	
	entry := &cacheEntry{
		msg:       msg.Copy(),
		expires:   expires,
		key:       key,
		frequency: 1,
	}
	
	// Если запись уже существует, обновляем её
	if elem, exists := shard.lruMap[key]; exists {
		shard.lruList.Remove(elem)
		delete(shard.lruMap, key)
		delete(shard.items, key)
	}
	
	// Добавляем новую запись
	elem := shard.lruList.PushFront(entry)
	shard.lruMap[key] = elem
	shard.items[key] = entry
}

func (c *DNSCache) setNegative(q dns.Question, rcode int) {
	key := fmt.Sprintf("%s-%d-%d-negative", q.Name, q.Qtype, q.Qclass)
	shard := c.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	c.ensureCapacity(shard)

	// Negative TTL для NXDOMAIN обычно меньше
	ttl := uint32(300) // 5 minutes for negative cache
	
	msg := new(dns.Msg)
	msg.SetRcode(&dns.Msg{Question: []dns.Question{q}}, rcode)
	
	expires := time.Now().Add(time.Duration(ttl) * time.Second)
	
	entry := &cacheEntry{
		msg:        msg,
		expires:    expires,
		key:        key,
		isNegative: true,
		frequency:  1,
	}
	
	if elem, exists := shard.lruMap[key]; exists {
		shard.lruList.Remove(elem)
		delete(shard.lruMap, key)
		delete(shard.items, key)
	}
	
	elem := shard.lruList.PushFront(entry)
	shard.lruMap[key] = elem
	shard.items[key] = entry
	
	atomic.AddUint64(&c.stats.Negative, 1)
}

func (c *DNSCache) ensureCapacity(shard *cacheShard) {
	maxShardSize := c.maxSize / len(c.shards)
	
	for len(shard.items) >= maxShardSize {
		c.evictOldest(shard)
	}
}

func (c *DNSCache) evictOldest(shard *cacheShard) {
	elem := shard.lruList.Back()
	if elem != nil {
		c.evictEntry(shard, elem)
	}
}

func (c *DNSCache) evictEntry(shard *cacheShard, elem *list.Element) {
	entry := elem.Value.(*cacheEntry)
	
	delete(shard.items, entry.key)
	delete(shard.lruMap, entry.key)
	shard.lruList.Remove(elem)
	
	atomic.AddUint64(&c.stats.Evictions, 1)
}

func (c *DNSCache) Stats() CacheStats {
	return CacheStats{
		Hits:      atomic.LoadUint64(&c.stats.Hits),
		Misses:    atomic.LoadUint64(&c.stats.Misses),
		Evictions: atomic.LoadUint64(&c.stats.Evictions),
		Expired:   atomic.LoadUint64(&c.stats.Expired),
		Negative:  atomic.LoadUint64(&c.stats.Negative),
	}
}

func (c *DNSCache) Size() int {
	total := 0
	for _, shard := range c.shards {
		shard.mu.RLock()
		total += len(shard.items)
		shard.mu.RUnlock()
	}
	return total
}

func (c *DNSCache) Clear() {
	for _, shard := range c.shards {
		shard.mu.Lock()
		shard.items = make(map[string]*cacheEntry)
		shard.lruList = list.New()
		shard.lruMap = make(map[string]*list.Element)
		shard.mu.Unlock()
	}
}

func (c *DNSCache) getAllDomains() []dns.Question {
	var domains []dns.Question
	
	for _, shard := range c.shards {
		shard.mu.RLock()
		for key, entry := range shard.items {
			if time.Now().Before(entry.expires) {
				// Парсим ключ обратно в Question
				parts := strings.Split(key, "-")
				if len(parts) == 3 {
					qtype, _ := strconv.Atoi(parts[1])
					qclass, _ := strconv.Atoi(parts[2])
					domains = append(domains, dns.Question{
						Name:   parts[0],
						Qtype:  uint16(qtype),
						Qclass: uint16(qclass),
					})
				}
			}
		}
		shard.mu.RUnlock()
	}
	
	return domains
}

func (s *Server) cacheCleaner() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.cache.cleanExpired()
	}
}

func (c *DNSCache) cleanExpired() {
	now := time.Now()
	
	for _, shard := range c.shards {
		shard.mu.Lock()
		
		for key, entry := range shard.items {
			if now.After(entry.expires) {
				if elem, exists := shard.lruMap[key]; exists {
					shard.lruList.Remove(elem)
					delete(shard.lruMap, key)
				}
				delete(shard.items, key)
				atomic.AddUint64(&c.stats.Expired, 1)
			}
		}
		
		shard.mu.Unlock()
	}
}