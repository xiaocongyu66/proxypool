package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Dreamacro/clash/adapter"
	C "github.com/Dreamacro/clash/constant"
	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

// SiteCheckTarget defines a website to check accessibility.
// Short is the abbreviation used in node names.
type SiteCheckTarget struct {
	Name   string
	Short  string
	URL    string
	Expect int // expected status code (0 = any 2xx/3xx)
}

// 检测目标网站列表：yt=gpt=grok=claude=脸书=推特=tg=网飞
var siteCheckTargets = []SiteCheckTarget{
	{Name: "YouTube", Short: "yt", URL: "https://www.youtube.com", Expect: 0},
	{Name: "ChatGPT", Short: "gpt", URL: "https://chat.openai.com", Expect: 0},
	{Name: "Grok", Short: "grok", URL: "https://x.ai", Expect: 0},
	{Name: "Claude", Short: "claude", URL: "https://claude.ai", Expect: 0},
	{Name: "Facebook", Short: "fb", URL: "https://www.facebook.com", Expect: 0},
	{Name: "Twitter", Short: "tw", URL: "https://x.com", Expect: 0},
	{Name: "Telegram", Short: "tg", URL: "https://web.telegram.org", Expect: 0},
	{Name: "Netflix", Short: "nf", URL: "https://www.netflix.com", Expect: 0},
}

const siteCheckTimeout = 8 * time.Second

// CheckSitesAll checks website accessibility for all proxies concurrently.
// Results are stored in ProxyStats[].Sites.
func CheckSitesAll(proxies []proxy.Proxy) {
	if ok := checkErrorProxies(proxies); !ok {
		return
	}
	numWorker := SpeedConn
	if numWorker <= 0 {
		numWorker = 50
	}
	if numWorker > 500 {
		numWorker = 500
	}
	doneCount := 0
	dcm := sync.Mutex{}

	pool := newSimplePool(numWorker)
	pool.waitCount(len(proxies))

	for _, p := range proxies {
		pp := p
		pool.submit(func() {
			defer pool.jobDone()
			sites := checkSitesForProxy(pp)
			if ps, ok := ProxyStats.Find(pp); ok {
				ps.Sites = sites
			} else {
				ProxyStats = append(ProxyStats, Stat{
					Id:    pp.Identifier(),
					Sites: sites,
				})
			}
			dcm.Lock()
			doneCount++
			progress := float64(doneCount) * 100 / float64(len(proxies))
			fmt.Printf("\r\t[%5.1f%% SITE]", progress)
			dcm.Unlock()
		})
	}
	pool.waitAll()
	fmt.Println()
	log.Infoln("Site accessibility check done for %d proxies", len(proxies))
}

func checkSitesForProxy(p proxy.Proxy) []string {
	pmap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(p.String()), &pmap); err != nil {
		return nil
	}
	portVal, ok := pmap["port"].(float64)
	if !ok {
		return nil
	}
	pmap["port"] = int(portVal)
	if p.TypeName() == "vmess" {
		if aid, ok := pmap["alterId"].(float64); ok {
			pmap["alterId"] = int(aid)
		}
	}

	if proxy.GoodNodeThatClashUnsupported(p) {
		return nil
	}

	clashProxy, err := adapter.ParseProxy(pmap)
	if err != nil {
		return nil
	}

	accessible := make([]string, 0)
	var mu sync.Mutex

	pool := newSimplePool(len(siteCheckTargets))
	pool.waitCount(len(siteCheckTargets))

	for _, target := range siteCheckTargets {
		t := target
		pool.submit(func() {
			defer pool.jobDone()
			if checkSingleSite(clashProxy, t) {
				mu.Lock()
				accessible = append(accessible, t.Short)
				mu.Unlock()
			}
		})
	}
	pool.waitAll()
	return accessible
}

func checkSingleSite(clashProxy C.Proxy, target SiteCheckTarget) bool {
	ctx, cancel := context.WithTimeout(context.Background(), siteCheckTimeout)
	defer cancel()

	addr, err := urlToMetadata(target.URL)
	if err != nil {
		return false
	}

	transport := &http.Transport{
		Dial: func(string, string) (net.Conn, error) {
			return clashProxy.DialContext(ctx, &addr)
		},
		ResponseHeaderTimeout: siteCheckTimeout,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   siteCheckTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, target.URL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	if resp.StatusCode == 0 {
		return false
	}
	if target.Expect == 0 {
		return resp.StatusCode >= 200 && resp.StatusCode < 500
	}
	return resp.StatusCode == target.Expect
}

var _ = log.Infoln
