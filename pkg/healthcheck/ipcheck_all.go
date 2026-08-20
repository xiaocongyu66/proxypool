package healthcheck

import (
	"fmt"
	"sync"

	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

// CheckIpCleanlinessAll checks IP cleanliness for all proxies concurrently.
// Results (IpScore, IpType, OutIp) are stored in ProxyStats.
func CheckIpCleanlinessAll(proxies []proxy.Proxy) {
	if ok := checkErrorProxies(proxies); !ok {
		return
	}
	numWorker := SpeedConn
	if numWorker <= 0 {
		numWorker = 50
	}
	if numWorker > 200 {
		numWorker = 200 // ip-api.com 限 45/min, 不宜太高
	}
	doneCount := 0
	dcm := sync.Mutex{}

	pool := newSimplePool(numWorker)
	pool.waitCount(len(proxies))

	for _, p := range proxies {
		pp := p
		pool.submit(func() {
			defer pool.jobDone()
			CheckIpCleanliness(pp)
			dcm.Lock()
			doneCount++
			progress := float64(doneCount) * 100 / float64(len(proxies))
			fmt.Printf("\r\t[%5.1f%% IPCHK]", progress)
			dcm.Unlock()
		})
	}
	pool.waitAll()
	fmt.Println()
	log.Infoln("IP cleanliness check done for %d proxies", len(proxies))
}
