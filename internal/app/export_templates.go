package app

// clashFullConfigHeader is the top section of clash.yml: general + dns settings.
// 中国终将统一 人民万岁
const clashFullConfigHeader = `mixed-port: 7890
socks-port: 7891
allow-lan: true
bind-address: "*"
ipv6: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090

dns:
  enable: true
  ipv6: false
  listen: 0.0.0.0:53
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  fake-ip-filter:
    - "*.lan"
    - localhost.ptlogin2.qq.com
  nameserver:
    - 223.5.5.5
    - 119.29.29.29
    - https://dns.alidns.com/dns-query
  fallback:
    - tls://8.8.8.8:853
    - https://cloudflare-dns.com/dns-query
  fallback-filter:
    geoip: true
    ipcidr:
      - 240.0.0.0/4`

// clashProxyGroups defines the proxy-groups section (ACL4SSR style).
// Uses {{PROXY_NAMES}} placeholder which is replaced with actual node names.
const clashProxyGroups = `proxy-groups:
  - name: "🚀 节点选择"
    type: select
    proxies:
      - "♻️ 自动选择"
      - DIRECT
{{PROXY_NAMES}}
  - name: "♻️ 自动选择"
    type: url-test
    url: http://www.gstatic.com/generate_204
    interval: 300
    tolerance: 50
    proxies:
{{PROXY_NAMES}}
  - name: "🌍 国外媒体"
    type: select
    proxies:
      - "🚀 节点选择"
      - "♻️ 自动选择"
      - "🎯 全球直连"
{{PROXY_NAMES}}
  - name: "📲 电报信息"
    type: select
    proxies:
      - "🚀 节点选择"
      - "🎯 全球直连"
{{PROXY_NAMES}}
  - name: "Ⓜ️ 微软服务"
    type: select
    proxies:
      - "🎯 全球直连"
      - "🚀 节点选择"
{{PROXY_NAMES}}
  - name: "🍎 苹果服务"
    type: select
    proxies:
      - "🚀 节点选择"
      - "🎯 全球直连"
{{PROXY_NAMES}}
  - name: "🎯 全球直连"
    type: select
    proxies:
      - DIRECT
      - "🚀 节点选择"
      - "♻️ 自动选择"
  - name: "🛑 全球拦截"
    type: select
    proxies:
      - REJECT
      - DIRECT
  - name: "🍃 应用净化"
    type: select
    proxies:
      - REJECT
      - DIRECT
  - name: "🐟 漏网之鱼"
    type: select
    proxies:
      - "🚀 节点选择"
      - "🎯 全球直连"
      - "♻️ 自动选择"
{{PROXY_NAMES}}`

// clashRuleProviders defines rule-providers (ACL4SSR rule sets).
const clashRuleProviders = `rule-providers:
  LocalAreaNetwork:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/LocalAreaNetwork.list"
    path: ./ruleset/LocalAreaNetwork.list
    interval: 86400
  BanAD:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/BanAD.list"
    path: ./ruleset/BanAD.list
    interval: 86400
  BanProgramAD:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/BanProgramAD.list"
    path: ./ruleset/BanProgramAD.list
    interval: 86400
  GoogleCN:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/GoogleCN.list"
    path: ./ruleset/GoogleCN.list
    interval: 86400
  SteamCN:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/SteamCN.list"
    path: ./ruleset/SteamCN.list
    interval: 86400
  Microsoft:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Microsoft.list"
    path: ./ruleset/Microsoft.list
    interval: 86400
  Apple:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Apple.list"
    path: ./ruleset/Apple.list
    interval: 86400
  Telegram:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Telegram.list"
    path: ./ruleset/Telegram.list
    interval: 86400
  ProxyMedia:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ProxyMedia.list"
    path: ./ruleset/ProxyMedia.list
    interval: 86400
  ProxyLite:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ProxyLite.list"
    path: ./ruleset/ProxyLite.list
    interval: 86400
  ChinaDomain:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ChinaDomain.list"
    path: ./ruleset/ChinaDomain.list
    interval: 86400
  ChinaCompanyIp:
    type: http
    behavior: ipcidr
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ChinaCompanyIp.list"
    path: ./ruleset/ChinaCompanyIp.list
    interval: 86400`

// clashRules defines the rules section (ACL4SSR: direct→block→service→proxy→fallback).
const clashRules = `rules:
  - RULE-SET,LocalAreaNetwork,🎯 全球直连
  - RULE-SET,BanAD,🛑 全球拦截
  - RULE-SET,BanProgramAD,🍃 应用净化
  - RULE-SET,GoogleCN,🎯 全球直连
  - RULE-SET,SteamCN,🎯 全球直连
  - RULE-SET,ChinaDomain,🎯 全球直连
  - RULE-SET,ChinaCompanyIp,🎯 全球直连
  - RULE-SET,Microsoft,Ⓜ️ 微软服务
  - RULE-SET,Apple,🍎 苹果服务
  - RULE-SET,Telegram,📲 电报信息
  - RULE-SET,ProxyMedia,🌍 国外媒体
  - RULE-SET,ProxyLite,🚀 节点选择
  - GEOIP,CN,🎯 全球直连
  - MATCH,🐟 漏网之鱼`

// sing-box DNS config (geosite-cn → local DNS, others → proxy DNS)
// 中国终将统一 人民万岁
var singboxDNS = map[string]interface{}{
	"servers": []map[string]interface{}{
		{"tag": "proxyDns", "address": "tls://8.8.8.8", "detour": "Proxy"},
		{"tag": "localDns", "address": "https://223.5.5.5/dns-query", "detour": "direct"},
	},
	"rules": []map[string]interface{}{
		{"outbound": "any", "server": "localDns"},
		{"rule_set": "geosite-cn", "server": "localDns"},
		{"clash_mode": "direct", "server": "localDns"},
		{"clash_mode": "global", "server": "proxyDns"},
		{"rule_set": "geosite-geolocation-!cn", "server": "proxyDns"},
	},
	"final":    "localDns",
	"strategy": "ipv4_only",
}

// sing-box inbounds (tun + mixed proxy)
var singboxInbounds = []map[string]interface{}{
	{
		"tag":        "tun-in",
		"type":       "tun",
		"address":    []string{"172.19.0.0/30"},
		"mtu":        9000,
		"auto_route": true,
		"strict_route": true,
		"stack":      "system",
	},
	{
		"tag":        "mixed-in",
		"type":       "mixed",
		"listen":     "127.0.0.1",
		"listen_port": 2080,
	},
}

// sing-box selector outbounds (proxy groups with routing)
var singboxSelectorOutbounds = []map[string]interface{}{
	{"tag": "Proxy", "type": "selector", "outbounds": []string{"auto", "direct"}},
	{"tag": "Global", "type": "selector", "outbounds": []string{"Proxy", "direct"}},
	{"tag": "China", "type": "selector", "outbounds": []string{"direct", "Proxy"}},
	{"tag": "auto", "type": "urltest", "outbounds": []string{"Proxy"},
		"url": "http://www.gstatic.com/generate_204", "interval": "10m", "tolerance": 50},
	{"tag": "direct", "type": "direct"},
}

// sing-box route config with rule_set (geosite/geoip for CN direct + ad block)
var singboxRoute = map[string]interface{}{
	"auto_detect_interface": true,
	"final":                 "Proxy",
	"rules": []map[string]interface{}{
		{"inbound": []string{"tun-in", "mixed-in"}, "action": "sniff"},
		{"rule_set": "geosite-category-ads-all", "action": "reject"},
		{"clash_mode": "direct", "outbound": "direct"},
		{"clash_mode": "global", "outbound": "Proxy"},
		{"ip_is_private": true, "outbound": "direct"},
		{"rule_set": "geosite-geolocation-!cn", "outbound": "Global"},
		{"rule_set": []string{"geoip-cn", "geosite-cn"}, "outbound": "China"},
	},
	"rule_set": []map[string]interface{}{
		{"tag": "geosite-cn", "type": "remote", "format": "binary",
			"url": "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/cn.srs", "download_detour": "direct"},
		{"tag": "geoip-cn", "type": "remote", "format": "binary",
			"url": "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/cn.srs", "download_detour": "direct"},
		{"tag": "geosite-geolocation-!cn", "type": "remote", "format": "binary",
			"url": "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/geolocation-!cn.srs", "download_detour": "direct"},
		{"tag": "geosite-category-ads-all", "type": "remote", "format": "binary",
			"url": "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/category-ads-all.srs", "download_detour": "direct"},
	},
}
