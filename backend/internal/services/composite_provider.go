package services

import (
	"sync"

	"smarttraffic/internal/models"
)

type CompositeRealtimeProvider struct {
	mu       sync.RWMutex
	primary  RealtimeStatsProvider
	fallback RealtimeStatsProvider
	active   RealtimeStatsProvider
}

func NewCompositeRealtimeProvider(primary, fallback RealtimeStatsProvider) *CompositeRealtimeProvider {
	return &CompositeRealtimeProvider{
		primary:  primary,
		fallback: fallback,
		active:   primary,
	}
}

func (c *CompositeRealtimeProvider) GetRealtimeStats() map[string]*models.PeerRealtimeStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active.GetRealtimeStats()
}

func (c *CompositeRealtimeProvider) IsAPIReachable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active.IsAPIReachable()
}

func (c *CompositeRealtimeProvider) SwitchToFallback() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = c.fallback
}

func (c *CompositeRealtimeProvider) SwitchToPrimary() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = c.primary
}

func (c *CompositeRealtimeProvider) Primary() RealtimeStatsProvider {
	return c.primary
}
