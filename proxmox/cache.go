package proxmox

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// Global Cache instance
var Cache *cache.Cache

// InitCache initializes the in-memory cache system
func InitCache() {
	// Create a cache with a default expiration time of 10 seconds, and which
	// purges expired items every 30 seconds
	Cache = cache.New(10*time.Second, 30*time.Second)
}
