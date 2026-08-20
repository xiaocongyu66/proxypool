package provider

import (
	"encoding/json"
	"strings"

	"github.com/ssrlive/proxypool/pkg/proxy"
)

// SingBox provides sing-box JSON output format
type SingBox struct {
	Base
}

type sbOutbound struct {
	Tag            string                 `json:"tag"`
	Type           string                 `json:"type"`
	Server         string                 `json:"server,omitempty"`
	ServerPort     int                    `json:"server_port,omitempty"`
	Method         string                 `json:"method,omitempty"`
	Password       string                 `json:"password,omitempty"`
	UUID           string                 `json:"uuid,omitempty"`
	AlterID        int                    `json:"alter_id,omitempty"`
	Cipher         string                 `json:"cipher,omitempty"`
	TLS            *sbTLS                 `json:"tls,omitempty"`
	Transport      *sbTransport           `json:"transport,omitempty"`
	Network        string                 `json:"network,omitempty"`
	Username       string                 `json:"username,omitempty"`
	SNI            string                 `json:"server_name,omitempty"`
	Flow           string                 `json:"flow,omitempty"`
	Obfs           string                 `json:"obfs,omitempty"`
	ObfsPassword   string                 `json:"obfs_password,omitempty"`
	UpMbps         int                    `json:"up_mbps,omitempty"`
	DownMbps       int                    `json:"down_mbps,omitempty"`
	ALPN           []string               `json:"alpn,omitempty"`
	Extra          map[string]interface{} `json:"-"`
}

type sbTLS struct {
	Enabled         bool     `json:"enabled"`
	ServerName      string   `json:"server_name,omitempty"`
	Insecure        bool     `json:"insecure,omitempty"`
	ALPN            []string `json:"alpn,omitempty"`
	Reality         *sbReality `json:"reality,omitempty"`
}

type sbReality struct {
	Enabled  bool   `json:"enabled"`
	PublicKey string `json:"public_key,omitempty"`
	ShortID  string `json:"short_id,omitempty"`
}

type sbTransport struct {
	Type        string            `json:"type"`
	Path        string            `json:"path,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
}

func (s SingBox) Provide() string {
	s.preFilter()
	var resultBuilder strings.Builder
	outbounds := make([]sbOutbound, 0, len(*s.Proxies))
	for _, p := range *s.Proxies {
		ob := proxyToSingBox(p)
		if ob != nil {
			outbounds = append(outbounds, *ob)
		}
	}
	root := map[string]interface{}{
		"outbounds": outbounds,
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return ""
	}
	resultBuilder.WriteString(string(data))
	return resultBuilder.String()
}

func proxyToSingBox(p proxy.Proxy) *sbOutbound {
	switch t := p.(type) {
	case *proxy.Shadowsocks:
		return &sbOutbound{
			Tag:        t.Name,
			Type:       "shadowsocks",
			Server:     t.Server,
			ServerPort: t.Port,
			Method:     t.Cipher,
			Password:   t.Password,
		}
	case *proxy.ShadowsocksR:
		return &sbOutbound{
			Tag:        t.Name,
			Type:       "shadowsocksr",
			Server:     t.Server,
			ServerPort: t.Port,
			Method:     t.Cipher,
			Password:   t.Password,
		}
	case *proxy.Vmess:
		ob := &sbOutbound{
			Tag:        t.Name,
			Type:       "vmess",
			Server:     t.Server,
			ServerPort: t.Port,
			UUID:       t.UUID,
			AlterID:    t.AlterID,
			Cipher:     t.Cipher,
		}
		if t.TLS {
			ob.TLS = &sbTLS{Enabled: true, ServerName: t.ServerName, Insecure: t.SkipCertVerify}
		}
		if t.Network == "ws" && t.WSOpts != nil {
			ob.Transport = &sbTransport{Type: "ws", Path: t.WSOpts.Path, Headers: t.WSOpts.Headers}
		}
		return ob
	case *proxy.Vless:
		ob := &sbOutbound{
			Tag:        t.Name,
			Type:       "vless",
			Server:     t.Server,
			ServerPort: t.Port,
			UUID:       t.UUID,
			Flow:       t.Flow,
		}
		if t.TLS {
			ob.TLS = &sbTLS{Enabled: true, ServerName: t.SNI, Insecure: t.SkipCertVerify, ALPN: t.ALPN}
		}
		if t.RealityOpts != nil {
			ob.TLS = &sbTLS{
				Enabled: true,
				ServerName: t.SNI,
				Reality: &sbReality{Enabled: true, PublicKey: t.RealityOpts.PublicKey, ShortID: t.RealityOpts.ShortID},
			}
		}
		if t.Network == "ws" && t.WSOpts != nil {
			ob.Transport = &sbTransport{Type: "ws", Path: t.WSOpts.Path, Headers: t.WSOpts.Headers}
		}
		if t.Network == "grpc" && t.GrpcOpts != nil {
			ob.Transport = &sbTransport{Type: "grpc", ServiceName: t.GrpcOpts.GrpcServiceName}
		}
		return ob
	case *proxy.Trojan:
		ob := &sbOutbound{
			Tag:        t.Name,
			Type:       "trojan",
			Server:     t.Server,
			ServerPort: t.Port,
			Password:   t.Password,
		}
		if t.SNI != "" {
			ob.TLS = &sbTLS{Enabled: true, ServerName: t.SNI, Insecure: t.SkipCertVerify, ALPN: t.ALPN}
		}
		return ob
	case *proxy.Hysteria2:
		ob := &sbOutbound{
			Tag:          t.Name,
			Type:         "hysteria2",
			Server:       t.Server,
			ServerPort:   t.Port,
			Password:     t.Password,
			Obfs:         t.Obfs,
			ObfsPassword: t.ObfsPassword,
			UpMbps:       t.Up,
			DownMbps:     t.Down,
		}
		if t.SNI != "" || t.SkipCertVerify {
			ob.TLS = &sbTLS{Enabled: true, ServerName: t.SNI, Insecure: t.SkipCertVerify, ALPN: t.ALPN}
		}
		return ob
	case *proxy.Http:
		ob := &sbOutbound{
			Tag:        t.Name,
			Type:       "http",
			Server:     t.Server,
			ServerPort: t.Port,
			Username:   t.Username,
		}
		if t.TLS {
			ob.TLS = &sbTLS{Enabled: true, Insecure: t.SkipCertVerify}
		}
		return ob
	case *proxy.Socks5:
		return &sbOutbound{
			Tag:        t.Name,
			Type:       "socks",
			Server:     t.Server,
			ServerPort: t.Port,
			Username:   t.Username,
		}
	}
	return nil
}
