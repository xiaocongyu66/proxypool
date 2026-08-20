package provider

import (
	"strings"

	"github.com/ssrlive/proxypool/pkg/proxy"
	"github.com/ssrlive/proxypool/pkg/tool"
)

// V2RaySub provides a v2ray base64 subscription (all proxy types as links)
type V2RaySub struct {
	Base
}

func (sub V2RaySub) Provide() string {
	sub.preFilter()
	var resultBuilder strings.Builder
	for _, p := range *sub.Proxies {
		link := p.Link()
		if link != "" {
			resultBuilder.WriteString(link + "\n")
		}
	}
	return tool.Base64EncodeString(resultBuilder.String(), false)
}

// VlessSub provides a vless-only subscription
type VlessSub struct {
	Base
}

func (sub VlessSub) Provide() string {
	sub.Types = "vless"
	sub.preFilter()
	var resultBuilder strings.Builder
	for _, p := range *sub.Proxies {
		link := p.Link()
		if link != "" {
			resultBuilder.WriteString(link + "\n")
		}
	}
	return tool.Base64EncodeString(resultBuilder.String(), false)
}

// Hysteria2Sub provides a hysteria2-only subscription
type Hysteria2Sub struct {
	Base
}

func (sub Hysteria2Sub) Provide() string {
	sub.Types = "hysteria2"
	sub.preFilter()
	var resultBuilder strings.Builder
	for _, p := range *sub.Proxies {
		link := p.Link()
		if link != "" {
			resultBuilder.WriteString(link + "\n")
		}
	}
	return tool.Base64EncodeString(resultBuilder.String(), false)
}

var _ = proxy.ProxyList{}
