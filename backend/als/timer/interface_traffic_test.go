package timer

import (
	"testing"
	"time"
)

func TestInterfaceCachesSnapshotIsConsistentCopy(t *testing.T) {
	interfaceCachesMu.Lock()
	InterfaceCaches = map[int]*InterfaceTrafficCache{
		7: {
			InterfaceName: "eth0",
			LastCacheTime: time.Unix(10, 0),
			Caches:        [][3]uint64{{10, 20, 30}},
			LastRx:        20,
			LastTx:        30,
		},
	}
	interfaceCachesMu.Unlock()

	snapshot := InterfaceCachesSnapshot()
	snapshot[7].Caches[0][1] = 999
	snapshot[7].InterfaceName = "changed"

	interfaceCachesMu.RLock()
	defer interfaceCachesMu.RUnlock()
	if InterfaceCaches[7].Caches[0][1] != 20 || InterfaceCaches[7].InterfaceName != "eth0" {
		t.Fatal("snapshot aliases the live interface cache")
	}
}
