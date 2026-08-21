package healthcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

// CheckIpCleanlinessBatch uses ip-api.com batch endpoint (100 IPs per request)
// to check IP cleanliness for all proxies. Uses server IP directly — no proxy
// connection needed. This is ~100x faster than per-proxy probing.
func CheckIpCleanlinessBatch(proxies []proxy.Proxy) {
	if len(proxies) == 0 {
		return
	}

	// Collect unique server IPs (resolve hostnames)
	ipToProxies := map[string][]proxy.Proxy{}
	ipList := make([]string, 0, len(proxies))
	for _, p := range proxies {
		server := p.BaseInfo().Server
		if server == "" {
			continue
		}
		// Resolve hostname to IP
		ip := server
		if net.ParseIP(server) == nil {
			ips, err := net.LookupIP(server)
			if err == nil && len(ips) > 0 {
				ip = ips[0].String()
			} else {
				continue // can't resolve, skip
			}
		}
		if _, exists := ipToProxies[ip]; !exists {
			ipList = append(ipList, ip)
		}
		ipToProxies[ip] = append(ipToProxies[ip], p)
	}
	log.Infoln("[ipcheck] %d unique IPs to check (from %d proxies)", len(ipList), len(proxies))

	// Batch query: POST http://ip-api.com/batch (max 100 per request, 15 per minute for batch)
	batchSize := 100
	doneCount := 0
	dcm := sync.Mutex{}

	for i := 0; i < len(ipList); i += batchSize {
		end := i + batchSize
		if end > len(ipList) {
			end = len(ipList)
		}
		batch := ipList[i:end]

		// POST batch to ip-api.com
		body, _ := json.Marshal(batch)
		req, _ := http.NewRequest(http.MethodPost,
			"http://ip-api.com/batch?fields=status,query,isp,org,as,mobile,proxy,hosting,countryCode",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Warnln("[ipcheck] batch %d-%d failed: %s", i, end, err.Error())
			// Rate limit: wait and retry once
			time.Sleep(5 * time.Second)
			resp, err = client.Do(req)
			if err != nil {
				continue
			}
		}

		var results []ipApiResponse
		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		// Apply results to proxies
		for _, info := range results {
			if info.Status != "success" {
				continue
			}
			ipType := classifyIpType(&info)
			ipScore := computeIpScore(&info, ipType)
			for _, p := range ipToProxies[info.Query] {
				if ps, ok := ProxyStats.Find(p); ok {
					ps.IpType = ipType
					ps.IpScore = ipScore
					ps.OutIp = info.Query
				} else {
					ProxyStats = append(ProxyStats, Stat{
						Id:      p.Identifier(),
						IpType:  ipType,
						IpScore: ipScore,
						OutIp:   info.Query,
					})
				}
			}
		}

		doneCount += len(batch)
		dcm.Lock()
		progress := float64(doneCount) * 100 / float64(len(ipList))
		fmt.Printf("\r\t[%5.1f%% IPCHK]", progress)
		dcm.Unlock()

		// ip-api.com batch: 15 requests per minute
		if i+batchSize < len(ipList) {
			time.Sleep(4 * time.Second)
		}
	}
	fmt.Println()
	log.Infoln("[ipcheck] IP cleanliness check done for %d IPs", len(ipList))
}
