package app

import (
	"fmt"
	"sync"
	"time"

	C "github.com/ssrlive/proxypool/config"
	"github.com/ssrlive/proxypool/internal/cache"
	"github.com/ssrlive/proxypool/internal/database"
	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/geoIp"
	"github.com/ssrlive/proxypool/pkg/healthcheck"
	"github.com/ssrlive/proxypool/pkg/provider"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

var location, _ = time.LoadLocation("Asia/Shanghai")

func CrawlGo() {
	wg := &sync.WaitGroup{}
	var pc = make(chan proxy.Proxy)
	for _, g := range Getters {
		wg.Add(1)
		go g.Get2ChanWG(pc, wg)
	}
	proxies := cache.GetProxies("allproxies")
	dbProxies := database.GetAllProxies()
	// Show last time result when launch
	if proxies == nil && dbProxies != nil {
		cache.SetProxies("proxies", dbProxies)
		cache.LastCrawlTime = "抓取中，已载入上次数据库数据"
		log.Infoln("Database: loaded")
	}
	if dbProxies != nil {
		proxies = dbProxies.UniqAppendProxyList(proxies)
	}
	if proxies == nil {
		proxies = make(proxy.ProxyList, 0)
	}

	go func() {
		wg.Wait()
		close(pc)
	}() // Note: 为何并发？可以一边抓取一边读取而非抓完再读
	// for 用于阻塞goroutine
	for p := range pc { // Note: pc关闭后不能发送数据可以读取剩余数据
		if p != nil {
			proxies = proxies.UniqAppendProxy(p)
		}
	}

	proxies.NameClear()
	proxies = proxies.Derive()
	log.Infoln("CrawlGo unique proxy count: %d", len(proxies))

	// Clean Clash unsupported proxy because health check depends on clash
	proxies = provider.Clash{
		Base: provider.Base{
			Proxies: &proxies,
		},
	}.CleanProxies()
	log.Infoln("CrawlGo clash supported proxy count: %d", len(proxies))

	cache.SetProxies("allproxies", proxies)
	cache.AllProxiesCount = proxies.Len()
	log.Infoln("AllProxiesCount: %d", cache.AllProxiesCount)
	cache.SSProxiesCount = proxies.TypeLen("ss")
	log.Infoln("SSProxiesCount: %d", cache.SSProxiesCount)
	cache.SSRProxiesCount = proxies.TypeLen("ssr")
	log.Infoln("SSRProxiesCount: %d", cache.SSRProxiesCount)
	cache.VmessProxiesCount = proxies.TypeLen("vmess")
	log.Infoln("VmessProxiesCount: %d", cache.VmessProxiesCount)
	cache.TrojanProxiesCount = proxies.TypeLen("trojan")
	log.Infoln("TrojanProxiesCount: %d", cache.TrojanProxiesCount)
	cache.VlessProxiesCount = proxies.TypeLen("vless")
	log.Infoln("VlessProxiesCount: %d", cache.VlessProxiesCount)
	cache.Hy2ProxiesCount = proxies.TypeLen("hysteria2")
	log.Infoln("Hy2ProxiesCount: %d", cache.Hy2ProxiesCount)
	cache.HttpProxiesCount = proxies.TypeLen("http")
	log.Infoln("HttpProxiesCount: %d", cache.HttpProxiesCount)
	cache.SocksProxiesCount = proxies.TypeLen("socks5")
	log.Infoln("SocksProxiesCount: %d", cache.SocksProxiesCount)
	cache.LastCrawlTime = time.Now().In(location).Format("2006-01-02 15:04:05")

	// === Stage 1: Fast TCP connectivity test (like SmartSub, no clash adapter) ===
	log.Infoln("Stage 1: Fast TCP connectivity test...")
	healthcheck.DelayConn = C.Config.HealthCheckConnection
	if healthcheck.DelayConn < 1000 {
		healthcheck.DelayConn = 1000
	}
	log.Infoln("CONF: Stage 1 concurrency=%d", healthcheck.DelayConn)

	proxies = healthcheck.TCPConnectTestAll(proxies)
	log.Infoln("Stage 1 done: TCP reachable proxy count: %d", len(proxies))

	// Skip clash-based delay test — TCP test already filtered unreachable nodes.
	// Format name like US_01 sorted by country
	proxies.NameAddCounrty().Sort()
	log.Infoln("Proxy rename DONE!")

	// Relay check and rename (skip if too few nodes)
	if len(proxies) > 0 && len(proxies) < 500 {
		healthcheck.RelayCheck(proxies)
		for i := range proxies {
			if s, ok := healthcheck.ProxyStats.Find(proxies[i]); ok {
				if s.Relay {
					_, c, e := geoIp.GeoIpDB.Find(s.OutIp)
					if e == nil {
						proxies[i].SetName(fmt.Sprintf("Relay_%s-%s", proxies[i].BaseInfo().Name, c))
					}
				} else if s.Pool {
					proxies[i].SetName(fmt.Sprintf("Pool_%s", proxies[i].BaseInfo().Name))
				}
			}
		}
	}

	// === Stage 2: IP cleanliness check (batch API, like sub2api) ===
	log.Infoln("Stage 2: IP cleanliness check (batch, no proxy connection)...")
	healthcheck.CheckIpCleanlinessBatch(proxies)
	
	// 淘汰脏 IP 节点：datacenter+proxy flagged (IpScore < 40) 直接淘汰
	beforeIpFilter := len(proxies)
	ipFiltered := make(proxy.ProxyList, 0, len(proxies))
	for _, p := range proxies {
		ps, ok := healthcheck.ProxyStats.Find(p)
		if !ok {
			ipFiltered = append(ipFiltered, p) // 没查到的保留
			continue
		}
		if ps.IpScore < 40 {
			continue // 脏 IP 淘汰
		}
		ipFiltered = append(ipFiltered, p)
	}
	proxies = ipFiltered
	log.Infoln("Stage 2 done: IP filter %d -> %d (removed %d dirty nodes)", beforeIpFilter, len(proxies), beforeIpFilter-len(proxies))

	// === Stage 3: Continuous speed test (high concurrency, with cap) ===
	log.Infoln("Stage 3: Continuous 10s speed test...")
	if C.Config.SpeedTest {
		cache.IsSpeedTest = "已开启"
		healthcheck.SpeedConn = C.Config.SpeedConnection
		if healthcheck.SpeedConn < 100 {
			healthcheck.SpeedConn = 100
		}
		if C.Config.SpeedTimeout > 0 {
			healthcheck.SpeedTimeout = time.Second * time.Duration(C.Config.SpeedTimeout)
		}
		log.Infoln("CONF: Stage 3 concurrency=%d, timeout=%s", healthcheck.SpeedConn, healthcheck.SpeedTimeout)
		proxies = healthcheck.SpeedTestContinuousAll(proxies)
	} else {
		cache.IsSpeedTest = "未开启"
	}

	// === Stage 4: Site accessibility + quality scoring ===
	log.Infoln("Stage 4: Site accessibility + quality scoring...")
	healthcheck.CheckSitesAll(proxies)
	healthcheck.ScoreAllProxies(proxies)

	// === Final filter: remove low-quality / useless nodes ===
	beforeFilter := len(proxies)
	filtered := make(proxy.ProxyList, 0, len(proxies))
	for _, p := range proxies {
		ps, ok := healthcheck.ProxyStats.Find(p)
		if !ok {
			continue
		}
		if ps.Quality < 30 {
			continue
		}
		if ps.Speed == 0 && len(ps.Sites) == 0 {
			continue
		}
		filtered = append(filtered, p)
	}
	proxies = filtered
	log.Infoln("Final filter: %d -> %d (removed %d useless nodes)", beforeFilter, len(proxies), beforeFilter-len(proxies))

	// Apply final names: Country + Speed + Sites + Score
	for i := range proxies {
		proxies[i].SetName(healthcheck.FormatProxyName(proxies[i], 60))
	}
	proxies.NameAddIndex()

	// 可用节点存储
	cache.SetProxies("proxies", proxies)
	cache.UsefullProxiesCount = proxies.Len()
	database.SaveProxyList(proxies)
	database.ClearOldItems()

	log.Infoln("Usablility checking done. Open %s to check", C.Config.HostUrl())

	// 测速
	speedTestNew(proxies)
	cache.SetString("clashproxies", provider.Clash{
		Base: provider.Base{
			Proxies: &proxies,
		},
	}.Provide()) // update static string provider
	cache.SetString("surgeproxies", provider.Surge{
		Base: provider.Base{
			Proxies: &proxies,
		},
	}.Provide())

	// Export subscription files to disk (clash.yml, v2ray.txt, singbox.json, etc.)
	ExportFiles()
}

// Speed test for new proxies
func speedTestNew(proxies proxy.ProxyList) {
	if C.Config.SpeedTest {
		cache.IsSpeedTest = "已开启"
		if C.Config.SpeedTimeout > 0 {
			healthcheck.SpeedTimeout = time.Second * time.Duration(C.Config.SpeedTimeout)
			log.Infoln("config: Speed test timeout is set to %d seconds", C.Config.SpeedTimeout)
		}
		healthcheck.SpeedTestNew(proxies)
	} else {
		cache.IsSpeedTest = "未开启"
	}
}

// Speed test for all proxies in proxy.ProxyList
func SpeedTest(proxies proxy.ProxyList) {
	if C.Config.SpeedTest {
		cache.IsSpeedTest = "已开启"
		if C.Config.SpeedTimeout > 0 {
			log.Infoln("config: Speed test timeout is set to %d seconds", C.Config.SpeedTimeout)
			healthcheck.SpeedTimeout = time.Second * time.Duration(C.Config.SpeedTimeout)
		}
		healthcheck.SpeedTestAll(proxies)
	} else {
		cache.IsSpeedTest = "未开启"
	}
}
