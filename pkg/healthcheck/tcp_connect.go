package healthcheck

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

// TCPConnectTestAll does a fast TCP handshake test for all proxies.
// This is much faster than full clash adapter delay test (like SmartSub approach).
// Returns only proxies whose server:port is TCP reachable.
func TCPConnectTestAll(proxies []proxy.Proxy) proxy.ProxyList {
	if len(proxies) == 0 {
		return proxies
	}
	numWorker := DelayConn
	if numWorker < 500 {
		numWorker = 500
	}
	if numWorker > 2000 {
		numWorker = 2000
	}
	timeout := 3 * time.Second

	result := make(proxy.ProxyList, 0, len(proxies)/2)
	m := sync.Mutex{}
	doneCount := 0
	dcm := sync.Mutex{}

	pool := newSimplePool(numWorker)

	for _, p := range proxies {
		pp := p
		pool.submit(func() {
			defer pool.jobDone()
			host := pp.BaseInfo().Server
			port := fmt.Sprintf("%d", pp.BaseInfo().Port)
			if host == "" || port == "0" {
				return
			}
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
			if err != nil {
				return
			}
			conn.Close()

			// TCP reachable — record a basic delay stat
			m.Lock()
			if ps, ok := ProxyStats.Find(pp); ok {
				ps.Delay = time.Second // placeholder, real delay tested later
			} else {
				ProxyStats = append(ProxyStats, Stat{
					Id:    pp.Identifier(),
					Delay: time.Second,
				})
			}
			result = append(result, pp)
			m.Unlock()

			dcm.Lock()
			doneCount++
			progress := float64(doneCount) * 100 / float64(len(proxies))
			fmt.Printf("\r\t[%5.1f%% TCP]", progress)
			dcm.Unlock()
		})
	}
	pool.waitAll()
	fmt.Println()
	log.Infoln("TCP connect test: %d -> %d reachable", len(proxies), len(result))
	return result
}
