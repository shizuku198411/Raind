package dns

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type CacheEntry struct {
	Msg       *dns.Msg
	ExpiresAt time.Time
}

func NewDnsCache(maxKeys int) *DnsCache {
	return &DnsCache{
		items:   make(map[string]CacheEntry),
		maxKeys: maxKeys,
	}
}

type DnsCache struct {
	mu      sync.RWMutex
	items   map[string]CacheEntry
	maxKeys int
}

func (c *DnsCache) Get(key string, now time.Time) (*dns.Msg, bool) {
	c.mu.RLock()
	ent, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(ent.ExpiresAt) {
		// delete expired entry
		c.mu.Lock()
		if ent2, ok2 := c.items[key]; ok2 && now.After(ent2.ExpiresAt) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return nil, false
	}
	return ent.Msg.Copy(), true
}

func (c *DnsCache) Put(key string, msg *dns.Msg, ttl time.Duration, now time.Time) {
	if ttl <= 0 {
		return
	}
	exp := now.Add(ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxKeys > 0 && len(c.items) >= c.maxKeys {
		for k := range c.items {
			delete(c.items, k)
			break
		}
	}
	c.items[key] = CacheEntry{
		Msg:       msg.Copy(),
		ExpiresAt: exp,
	}
}

func cacheKey(q dns.Question, dnssecOk bool) string {
	name := strings.ToLower(dns.Fqdn(q.Name))
	return fmt.Sprintf("%s|%d|%d|do=%t", name, q.Qtype, q.Qclass, dnssecOk)
}

func minTTL(msg *dns.Msg) (time.Duration, bool) {
	ttl := uint32(0)
	set := false
	for _, rr := range append(append(msg.Answer, msg.Ns...), msg.Extra...) {
		h := rr.Header()
		if h == nil {
			continue
		}
		if !set || h.Ttl < ttl {
			ttl = h.Ttl
			set = true
		}
	}
	if !set {
		return 0, false
	}
	return time.Duration(ttl) * time.Second, true
}

func isCacheableQuery(r *dns.Msg) bool {
	if r == nil || r.Opcode != dns.OpcodeQuery || r.Response {
		return false
	}
	if len(r.Question) != 1 {
		return false
	}
	return true
}

func doBit(r *dns.Msg) bool {
	edns := r.IsEdns0()
	if edns == nil {
		return false
	}
	return edns.Do()
}
