package resolver

import "time"

// Optimized configuration for faster DNS resolution
const (
	// Ultra-fast timeouts for aggressive optimization
	OptimizedTimeoutUDP = 30 * time.Millisecond
	OptimizedTimeoutTCP = 100 * time.Millisecond
	
	// Reduced TTL for faster cache updates
	OptimizedMaxAllowedTTL = uint32(60 * 60 * 6) // 6 hours instead of 48
	
	// Increased concurrent workers
	OptimizedMaxQueriesPerRequest = uint32(50) // Reduced for faster timeouts
	
	// Aggressive nameserver selection
	OptimizedDesireNumberOfNameserversPerZone = 5
	
	// Enable lazy enrichment for speed
	OptimizedLazyEnrichment = true
	
	// DNS settings
	OptimizedSuppressBogusResponseSections = true
	OptimizedRemoveAuthoritySectionForPositiveAnswers = true
	OptimizedRemoveAdditionalSectionForPositiveAnswers = true
)

// ApplyOptimizedConfig applies aggressive optimizations
func ApplyOptimizedConfig() {
	// Cache settings
	MaxAllowedTTL = OptimizedMaxAllowedTTL
	
	// Query limits
	MaxQueriesPerRequest = OptimizedMaxQueriesPerRequest
	
	// Nameserver settings
	DesireNumberOfNameserversPerZone = OptimizedDesireNumberOfNameserversPerZone
	LazyEnrichment = OptimizedLazyEnrichment
	
	// Response optimization
	SuppressBogusResponseSections = OptimizedSuppressBogusResponseSections
	RemoveAuthoritySectionForPositiveAnswers = OptimizedRemoveAuthoritySectionForPositiveAnswers
	RemoveAdditionalSectionForPositiveAnswers = OptimizedRemoveAdditionalSectionForPositiveAnswers
}

// ApplyUltraFastConfig - максимальная скорость
func ApplyUltraFastConfig() {
	// Cache settings
	MaxAllowedTTL = uint32(60 * 60 * 2) // 2 hours
	MaxQueriesPerRequest = 25
	
	// Nameserver settings
	LazyEnrichment = true
	DesireNumberOfNameserversPerZone = 3
}

// ApplyBalancedConfig - баланс между скоростью и надежностью
func ApplyBalancedConfig() {
	MaxQueriesPerRequest = 75
	MaxAllowedTTL = uint32(60 * 60 * 12) // 12 hours
	DesireNumberOfNameserversPerZone = 4
}