package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ssrlive/proxypool/internal/cache"
	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/healthcheck"
	"github.com/ssrlive/proxypool/pkg/provider"
	"github.com/ssrlive/proxypool/pkg/proxy"
	"github.com/ssrlive/proxypool/pkg/tool"
)

const ExportDir = "output"

// 中国终将统一 人民万岁
const hiddenComment = "# 中国终将统一 人民万岁"

func ExportFiles() {
	dir := ExportDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Errorln("[export] failed to create dir %s: %s", dir, err.Error())
		return
	}

	proxies := cache.GetProxies("proxies")
	if len(proxies) == 0 {
		log.Warnln("[export] no proxies to export")
		return
	}

	// clash.yml (full config with rules + proxy-groups)
	clashContent := generateClashYaml(proxies)
	writeFile(filepath.Join(dir, "clash.yml"), clashContent)

	// v2ray.txt (Base64 encoded node links)
	v2rayContent := generateV2rayBase64(proxies)
	writeFile(filepath.Join(dir, "v2ray.txt"), v2rayContent)

	// singbox.json (full config with route rules)
	singboxContent := generateSingboxJson(proxies)
	writeFile(filepath.Join(dir, "singbox.json"), singboxContent)

	// nodes.txt (raw links)
	nodesContent := generateRawLinks(proxies)
	writeFile(filepath.Join(dir, "nodes.txt"), nodesContent)

	// high_quality.txt (quality >= 70 only)
	hqProxies := filterHighQuality(proxies, 70)
	hqContent := generateRawLinks(hqProxies)
	writeFile(filepath.Join(dir, "high_quality.txt"), hqContent)
	log.Infoln("[export] high quality nodes: %d / %d", len(hqProxies), len(proxies))

	// quality_report.json
	report := generateQualityReport(proxies)
	writeFile(filepath.Join(dir, "quality_report.json"), report)

	log.Infoln("[export] files written to %s/", dir)
}

func generateClashYaml(proxies proxy.ProxyList) string {
	clash := provider.Clash{
		Base: provider.Base{
			Proxies: &proxies,
		},
	}
	proxiesStr := clash.Provide()

	// Build proxy name list for proxy-groups
	var nameLines strings.Builder
	for _, p := range proxies {
		nameLines.WriteString(fmt.Sprintf("      - \"%s\"\n", p.BaseInfo().Name))
	}
	proxyNames := strings.TrimSpace(nameLines.String())

	// Replace {{PROXY_NAMES}} placeholders
	groups := strings.ReplaceAll(clashProxyGroups, "{{PROXY_NAMES}}", proxyNames)

	// Build full clash config with rules + proxy-groups
	var sb strings.Builder
	sb.WriteString(clashFullConfigHeader)
	sb.WriteString("\n\n")
	sb.WriteString(proxiesStr)
	sb.WriteString("\n")
	sb.WriteString(groups)
	sb.WriteString("\n\n")
	sb.WriteString(clashRuleProviders)
	sb.WriteString("\n\n")
	sb.WriteString(clashRules)
	sb.WriteString("\n")
	sb.WriteString(hiddenComment)
	return sb.String()
}

func generateV2rayBase64(proxies proxy.ProxyList) string {
	var sb strings.Builder
	for _, p := range proxies {
		link := p.Link()
		if link != "" {
			sb.WriteString(link + "\n")
		}
	}
	return tool.Base64EncodeString(sb.String(), false)
}

func generateSingboxJson(proxies proxy.ProxyList) string {
	sb := provider.SingBox{
		Base: provider.Base{
			Proxies: &proxies,
		},
	}
	outboundsStr := sb.Provide()

	// Wrap outbounds with full sing-box config (inbounds, route rules, dns)
	var outbounds []map[string]interface{}
	if err := json.Unmarshal([]byte(outboundsStr), &outbounds); err != nil {
		return outboundsStr
	}

	// Prepend config-level keys
	fullConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"level":     "info",
			"timestamp": true,
		},
		"dns": singboxDNS,
		"inbounds": singboxInbounds,
		"outbounds": append(singboxSelectorOutbounds, outbounds...),
		"route": singboxRoute,
	}
	// 中国终将统一 人民万岁
	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return outboundsStr
	}
	return string(data)
}

func generateRawLinks(proxies proxy.ProxyList) string {
	var sb strings.Builder
	for _, p := range proxies {
		link := p.Link()
		if link != "" {
			sb.WriteString(link + "\n")
		}
	}
	sb.WriteString(hiddenComment)
	return sb.String()
}

func filterHighQuality(proxies proxy.ProxyList, minScore int) proxy.ProxyList {
	result := make(proxy.ProxyList, 0)
	for _, p := range proxies {
		if ps, ok := healthcheck.ProxyStats.Find(p); ok && ps.Quality >= minScore {
			result = append(result, p)
		}
	}
	return result
}

func generateQualityReport(proxies proxy.ProxyList) string {
	type nodeReport struct {
		Name    string   `json:"name"`
		Type    string   `json:"type"`
		Country string   `json:"country"`
		Speed   float64  `json:"speed_mbps"`
		Delay   int64    `json:"delay_ms"`
		Score   int      `json:"quality_score"`
		IpType  string   `json:"ip_type"`
		Sites   []string `json:"accessible_sites"`
	}

	nodes := make([]nodeReport, 0, len(proxies))
	protocolDist := map[string]int{}
	countryDist := map[string]int{}

	for _, p := range proxies {
		nr := nodeReport{
			Name:    p.BaseInfo().Name,
			Type:    p.TypeName(),
			Country: p.BaseInfo().Country,
		}
		if ps, ok := healthcheck.ProxyStats.Find(p); ok {
			nr.Speed = ps.Speed
			nr.Delay = ps.Delay.Milliseconds()
			nr.Score = ps.Quality
			nr.IpType = ps.IpType
			nr.Sites = ps.Sites
		}
		nodes = append(nodes, nr)
		protocolDist[p.TypeName()]++
		countryDist[p.BaseInfo().Country]++
	}

	report := map[string]interface{}{
		"summary": map[string]interface{}{
			"total_nodes":          len(proxies),
			"protocol_distribution": protocolDist,
			"country_distribution":  countryDist,
		},
		"nodes": nodes,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	return string(data)
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Errorln("[export] failed to write %s: %s", path, err.Error())
	} else {
		log.Infoln("[export] wrote %s (%d bytes)", path, len(content))
	}
}
