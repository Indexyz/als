package timer

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samlm0/als/v2/als/client"
	"github.com/vishvananda/netlink"
)

type InterfaceTrafficCache struct {
	InterfaceName string
	LastCacheTime time.Time
	Caches        [][3]uint64
	LastRx        uint64 `json:"-"`
	LastTx        uint64 `json:"-"`
}

var InterfaceCaches = make(map[int]*InterfaceTrafficCache)
var interfaceCachesMu sync.RWMutex

// InterfaceCachesSnapshot returns a consistent copy for readers such as API handlers.
func InterfaceCachesSnapshot() map[int]*InterfaceTrafficCache {
	interfaceCachesMu.RLock()
	defer interfaceCachesMu.RUnlock()

	snapshot := make(map[int]*InterfaceTrafficCache, len(InterfaceCaches))
	for index, cache := range InterfaceCaches {
		if cache == nil {
			snapshot[index] = nil
			continue
		}
		copyCache := *cache
		copyCache.Caches = append([][3]uint64(nil), cache.Caches...)
		snapshot[index] = &copyCache
	}
	return snapshot
}

func SetupInterfaceBroadcast() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		interfaces, err := net.Interfaces()
		if err != nil {
			continue
		}

		for _, iface := range interfaces {
			// skip down interface
			if iface.Flags&net.FlagUp == 0 {
				continue
			}

			// skip docker
			if strings.Index(iface.Name, "docker") == 0 {
				continue
			}

			// skip lo
			if iface.Name == "lo" {
				continue
			}

			// skip wireguard
			if strings.Index(iface.Name, "wt") == 0 {
				continue
			}

			// skip veth
			if strings.Index(iface.Name, "veth") == 0 {
				continue
			}

			link, err := netlink.LinkByIndex(iface.Index)
			if err != nil || link.Attrs() == nil || link.Attrs().Statistics == nil {
				continue
			}
			now := time.Now()

			interfaceCachesMu.Lock()
			cache, ok := InterfaceCaches[iface.Index]
			if !ok {
				cache = &InterfaceTrafficCache{InterfaceName: iface.Name, Caches: make([][3]uint64, 0)}
				InterfaceCaches[iface.Index] = cache
			}
			cache.LastRx = link.Attrs().Statistics.RxBytes
			cache.LastTx = link.Attrs().Statistics.TxBytes
			cache.Caches = append(cache.Caches, [3]uint64{uint64(now.Unix()), cache.LastRx, cache.LastTx})
			if len(cache.Caches) > 30 {
				cache.Caches = cache.Caches[len(cache.Caches)-30:]
			}
			cache.LastCacheTime = now
			lastRx, lastTx := cache.LastRx, cache.LastTx
			interfaceCachesMu.Unlock()

			client.BroadCastMessage("InterfaceTraffic", iface.Name+","+strconv.Itoa(int(now.Unix()))+","+strconv.Itoa(int(lastRx))+","+strconv.Itoa(int(lastTx)))
		}
	}
}
