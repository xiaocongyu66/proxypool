package healthcheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

// ipApiResponse represents the response from ip-api.com (free, no key needed, 45 req/min)
type ipApiResponse struct {
	Status      string `json:"status"`
	Query       string `json:"query"`
	ISP         string `json:"isp"`
	Org         string `json:"org"`
	AS          string `json:"as"`
	Mobile      bool   `json:"mobile"`
	Proxy       bool   `json:"proxy"`
	Hosting     bool   `json:"hosting"`
	CountryCode string `json:"countryCode"`
}

// CheckIpCleanliness probes the exit IP of a proxy and determines its cleanliness.
// Uses ip-api.com's JSON endpoint (free, no API key, 45 req/min limit).
// Results: IpScore (0-100), IpType (residential/mobile/datacenter).
func CheckIpCleanliness(p proxy.Proxy) {
	pmap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(p.String()), &pmap); err != nil {
		return
	}
	if portVal, ok := pmap["port"].(float64); ok {
		pmap["port"] = int(portVal)
	} else {
		return
	}
	if p.TypeName() == "vmess" {
		if aid, ok := pmap["alterId"].(float64); ok {
			pmap["alterId"] = int(aid)
		}
	}

	exitIP := probeExitIP(pmap, p)
	if exitIP == "" {
		return
	}

	ipInfo, err := fetchIpInfo(exitIP)
	if err != nil {
		log.Debugln("[ipcheck] failed to check IP %s: %s", exitIP, err.Error())
		return
	}

	ps, ok := ProxyStats.Find(p)
	if !ok {
		ps = &Stat{Id: p.Identifier()}
		ProxyStats = append(ProxyStats, *ps)
		ps, _ = ProxyStats.Find(p)
	}
	ps.OutIp = exitIP
	ps.IpType = classifyIpType(ipInfo)
	ps.IpScore = computeIpScore(ipInfo, ps.IpType)
}

// probeExitIP connects through the proxy and queries an IP echo service.
func probeExitIP(pmap map[string]interface{}, p proxy.Proxy) string {
	// Use clash adapter to establish connection and get exit IP
	clashProxy, err := parseClashProxy(pmap, p)
	if err != nil {
		return ""
	}

	ctx, cancel := contextWithTimeout(8 * time.Second)
	defer cancel()

	// Use httpbin/ipify to get exit IP
	addr, err := urlToMetadata("https://api.ipify.org?format=json")
	if err != nil {
		return ""
	}
	conn, err := clashProxy.DialContext(ctx, &addr)
	if err != nil {
		return ""
	}
	defer conn.Close()

	transport := newProxyTransport(clashProxy, addr, 8*time.Second)
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	var ipResult struct {
		IP string `json:"ip"`
	}
	if json.Unmarshal(body, &ipResult) == nil {
		return ipResult.IP
	}
	return ""
}

// fetchIpInfo queries ip-api.com for IP reputation info.
func fetchIpInfo(ip string) (*ipApiResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := "http://ip-api.com/json/" + ip + "?fields=status,query,isp,org,as,mobile,proxy,hosting,countryCode"
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info ipApiResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	if info.Status != "success" {
		return nil, fmt.Errorf("ip-api returned status: %s", info.Status)
	}
	return &info, nil
}

// classifyIpType determines if IP is residential, mobile, or datacenter.
func classifyIpType(info *ipApiResponse) string {
	if info.Mobile {
		return "mobile"
	}
	if info.Hosting {
		return "datacenter"
	}
	if info.Proxy {
		return "datacenter"
	}
	return "residential"
}

// computeIpScore computes 0-100 based on IP cleanliness.
//   - residential: 100 (best)
//   - mobile: 90
//   - datacenter (non-proxy): 60
//   - datacenter + proxy flagged: 30 (dirty)
func computeIpScore(info *ipApiResponse, ipType string) int {
	switch ipType {
	case "residential":
		return 100
	case "mobile":
		return 90
	case "datacenter":
		if info.Proxy {
			return 30
		}
		return 60
	default:
		return 50
	}
}

// ScoreAllProxies computes a comprehensive 0-100 quality score for each proxy,
// combining: IP cleanliness, delay, speed, stability, and site accessibility.
func ScoreAllProxies(proxies []proxy.Proxy) {
	for _, p := range proxies {
		ps, ok := ProxyStats.Find(p)
		if !ok {
			continue
		}
		ps.Quality = computeScore(ps)
	}
	log.Infoln("Quality scoring done for %d proxies", len(proxies))
}

// computeScore returns a comprehensive 0-100 quality score.
// Weighting (total 100):
//   - IP cleanliness: 30% (residential=30, mobile=27, dc=18, dirty=9)
//   - Speed: 25% (>20Mb=25, >10Mb=20, >5Mb=15, >1Mb=8, <1Mb=3)
//   - Delay: 15% (<500ms=15, <1s=12, <3s=8, <5s=4, >5s=0)
//   - Stability: 15% (stable=15, unstable=5)
//   - Site access: 15% (per site ~1.9, capped at 15)
func computeScore(ps *Stat) int {
	score := 0

	// IP cleanliness (30 points)
	switch ps.IpType {
	case "residential":
		score += 30
	case "mobile":
		score += 27
	case "datacenter":
		if ps.IpScore < 50 {
			score += 9
		} else {
			score += 18
		}
	default:
		score += 15
	}

	// Speed (25 points)
	speed := ps.Speed
	switch {
	case speed > 20:
		score += 25
	case speed > 10:
		score += 20
	case speed > 5:
		score += 15
	case speed > 1:
		score += 8
	case speed > 0:
		score += 3
	}

	// Delay (15 points)
	delayMs := ps.Delay.Milliseconds()
	switch {
	case delayMs < 500:
		score += 15
	case delayMs < 1000:
		score += 12
	case delayMs < 3000:
		score += 8
	case delayMs < 5000:
		score += 4
	}

	// Stability (15 points)
	if ps.Stable {
		score += 15
	} else {
		score += 5
	}

	// Site access (15 points, ~1.9 per site)
	siteCount := len(ps.Sites)
	siteScore := siteCount * 15 / 8
	if siteScore > 15 {
		siteScore = 15
	}
	score += siteScore

	if score > 100 {
		score = 100
	}
	return score
}

// countryCodeToName maps ISO country codes to Chinese names.
var countryCodeToName = map[string]string{
	"US": "美国", "CN": "中国", "JP": "日本", "KR": "韩国", "HK": "香港",
	"TW": "台湾", "SG": "新加坡", "GB": "英国", "DE": "德国", "FR": "法国",
	"CA": "加拿大", "AU": "澳大利亚", "NL": "荷兰", "RU": "俄罗斯", "IN": "印度",
	"BR": "巴西", "TH": "泰国", "VN": "越南", "MY": "马来西亚", "ID": "印尼",
	"PH": "菲律宾", "TR": "土耳其", "IT": "意大利", "ES": "西班牙", "CH": "瑞士",
	"SE": "瑞典", "NO": "挪威", "FI": "芬兰", "DK": "丹麦", "BE": "比利时",
	"AT": "奥地利", "IE": "爱尔兰", "PT": "葡萄牙", "PL": "波兰", "CZ": "捷克",
	"RO": "罗马尼亚", "UA": "乌克兰", "BG": "保加利亚", "RS": "塞尔维亚",
	"HR": "克罗地亚", "SK": "斯洛伐克", "HU": "匈牙利", "GR": "希腊", "IL": "以色列",
	"AE": "阿联酋", "SA": "沙特", "EG": "埃及", "ZA": "南非", "AR": "阿根廷",
	"CL": "智利", "CO": "哥伦比亚", "PE": "秘鲁", "MX": "墨西哥", "NZ": "新西兰",
}

// formatCountry extracts emoji flag and Chinese name from country string.
// Input format from GeoIP: "🇺🇸US" → returns "🇺🇸 美国"
func formatCountry(country string) string {
	if country == "" || country == "🏁ZZ" {
		return "🌐 未知"
	}
	// Extract emoji (first runes that are flag emojis) and code
	var emoji, code string
	runes := []rune(country)
	for i, r := range runes {
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			emoji += string(r)
		} else {
			code = string(runes[i:])
			break
		}
	}
	if code == "" {
		return country
	}
	if name, ok := countryCodeToName[code]; ok {
		return emoji + " " + name
	}
	return emoji + " " + code
}

// FormatProxyName generates a node name: 🇺🇸 美国 25.3Mb yt gpt tw S88
func FormatProxyName(p proxy.Proxy, maxLen int) string {
	ps, ok := ProxyStats.Find(p)
	if !ok {
		return p.BaseInfo().Name
	}

	country := formatCountry(p.BaseInfo().Country)

	// Speed: show as Mb
	speedStr := ""
	if ps.Speed > 0 {
		speedStr = fmt.Sprintf("%.1fMb", ps.Speed)
	}

	// Sites
	sitesStr := strings.Join(ps.Sites, " ")

	// Score
	scoreStr := fmt.Sprintf("S%d", ps.Quality)

	name := fmt.Sprintf("%s %s %s %s", country, speedStr, sitesStr, scoreStr)
	name = strings.ReplaceAll(name, "  ", " ")
	name = strings.TrimSpace(name)

	// Trim if too long: remove sites first
	if maxLen > 0 && len(name) > maxLen {
		name = fmt.Sprintf("%s %s %s", country, speedStr, scoreStr)
		name = strings.TrimSpace(name)
	}
	return name
}
