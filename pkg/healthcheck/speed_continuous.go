package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Dreamacro/clash/adapter"
	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

// 持续测速下载测试 URL（使用 Cloudflare 的 speed test 端点，全球可达、稳定、低延迟）
const speedTestDownloadURL = "https://speed.cloudflare.com/__down?bytes="
const speedTestDuration = 10 * time.Second
const speedTestMaxBytes = 25 * 1024 * 1024 // 25MB cap to limit bandwidth

// SpeedTestContinuousAll runs a 10-second continuous download test for all proxies.
// Results (Mbps) are stored in ProxyStats. Unstable proxies are filtered out.
func SpeedTestContinuousAll(proxies []proxy.Proxy) proxy.ProxyList {
	return speedTestContinuous(proxies, false)
}

// SpeedTestContinuousNew runs continuous speed test only for proxies not yet tested.
func SpeedTestContinuousNew(proxies []proxy.Proxy) proxy.ProxyList {
	return speedTestContinuous(proxies, true)
}

func speedTestContinuous(proxies []proxy.Proxy, onlyNew bool) proxy.ProxyList {
	SpeedExist = true
	if ok := checkErrorProxies(proxies); !ok {
		return proxies
	}
	numWorker := SpeedConn
	if numWorker <= 0 {
		numWorker = 5
	}
	if numWorker > 10 {
		numWorker = 10 // cap concurrency to save bandwidth
	}
	result := make(proxy.ProxyList, 0, len(proxies))
	m := sync.Mutex{}
	doneCount := 0
	dcm := sync.Mutex{}

	pool := newSimplePool(numWorker)
	pool.waitCount(len(proxies))

	for _, p := range proxies {
		pp := p
		pool.submit(func() {
			defer pool.jobDone()
			speed, stable := continuousDownloadTest(pp)
			m.Lock()
			if ps, ok := ProxyStats.Find(pp); ok {
				ps.UpdatePSSpeed(speed)
				ps.Stable = stable
			} else {
				ProxyStats = append(ProxyStats, Stat{
					Id:     pp.Identifier(),
					Speed:  speed,
					Stable: stable,
				})
			}
			if stable && speed > 0 {
				result = append(result, pp)
			}
			m.Unlock()
			dcm.Lock()
			doneCount++
			progress := float64(doneCount) * 100 / float64(len(proxies))
			fmt.Printf("\r\t[%5.1f%% SPEED]", progress)
			dcm.Unlock()
		})
	}
	pool.waitAll()
	fmt.Println()
	log.Infoln("Continuous speed test done. Stable count: %d / %d", len(result), len(proxies))
	return result
}

// continuousDownloadTest downloads from a speed test endpoint for up to 10 seconds.
// Returns (speedMbps, isStable). Stability is judged by consistent throughput.
func continuousDownloadTest(p proxy.Proxy) (speedMbps float64, stable bool) {
	pmap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(p.String()), &pmap); err != nil {
		return 0, false
	}
	portVal, ok := pmap["port"].(float64)
	if !ok {
		return 0, false
	}
	pmap["port"] = int(portVal)
	if p.TypeName() == "vmess" {
		if aid, ok := pmap["alterId"].(float64); ok {
			pmap["alterId"] = int(aid)
		}
	}

	if proxy.GoodNodeThatClashUnsupported(p) {
		host, _ := pmap["server"].(string)
		port := fmt.Sprint(pmap["port"].(int))
		if _, interval, err := netConnectivity(host, port); err == nil {
			return float64(interval.Milliseconds()), true
		}
		return 0, false
	}

	clashProxy, err := adapter.ParseProxy(pmap)
	if err != nil {
		return 0, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), speedTestDuration+5*time.Second)
	defer cancel()

	addr, err := urlToMetadata("https://speed.cloudflare.com/__down?bytes=1000")
	if err != nil {
		return 0, false
	}
	conn, err := clashProxy.DialContext(ctx, &addr)
	if err != nil {
		return 0, false
	}
	defer conn.Close()

	// Download increasing chunks for 10 seconds, measure throughput
	testURL := speedTestDownloadURL + fmt.Sprintf("%d", speedTestMaxBytes)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return 0, false
	}
	transport := &http.Transport{
		Dial: func(string, string) (net.Conn, error) {
			return clashProxy.DialContext(ctx, &addr)
		},
	}
	client := &http.Client{Transport: transport, Timeout: speedTestDuration + 2*time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	start := time.Now()
	totalBytes := int64(0)
	buf := make([]byte, 32*1024)
	samples := make([]float64, 0)
	lastSampleTime := start
	lastSampleBytes := int64(0)

	for {
		n, err := io.ReadFull(resp.Body, buf)
		if n > 0 {
			totalBytes += int64(n)
		}
		now := time.Now()
		elapsed := now.Sub(start)
		if now.Sub(lastSampleTime) >= 2*time.Second {
			chunkBytes := totalBytes - lastSampleBytes
			chunkTime := now.Sub(lastSampleTime).Seconds()
			if chunkTime > 0 {
				samples = append(samples, float64(chunkBytes)*8/chunkTime/1e6)
			}
			lastSampleBytes = totalBytes
			lastSampleTime = now
		}
		if err != nil {
			break
		}
		if elapsed >= speedTestDuration {
			break
		}
		if totalBytes >= int64(speedTestMaxBytes) {
			break
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed < 1 {
		return 0, false
	}
	speedMbps = float64(totalBytes) * 8 / elapsed / 1e6

	// Stability: check variance of samples. If samples differ by >50% from avg, unstable.
	if len(samples) >= 2 {
		avg := 0.0
		for _, s := range samples {
			avg += s
		}
		avg /= float64(len(samples))
		if avg > 0 {
			maxDev := 0.0
			for _, s := range samples {
				dev := (s - avg) / avg
				if dev < 0 {
					dev = -dev
				}
				if dev > maxDev {
					maxDev = dev
				}
			}
			stable = maxDev < 0.5 // <50% deviation = stable
		} else {
			stable = speedMbps > 0
		}
	} else {
		stable = speedMbps > 0
	}
	return speedMbps, stable
}

// simplePool is a lightweight goroutine pool to avoid grpool overhead
type simplePool struct {
	wg      sync.WaitGroup
	workers chan func()
}

func newSimplePool(n int) *simplePool {
	p := &simplePool{
		workers: make(chan func(), 100),
	}
	p.wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer p.wg.Done()
			for job := range p.workers {
				job()
			}
		}()
	}
	return p
}

func (p *simplePool) submit(f func()) {
	p.wg.Add(1)
	p.workers <- func() {
		defer p.wg.Done()
		f()
	}
}

func (p *simplePool) jobDone() {}

func (p *simplePool) waitCount(n int) {}

func (p *simplePool) waitAll() {
	close(p.workers)
	p.wg.Wait()
}

var _ = errors.New
