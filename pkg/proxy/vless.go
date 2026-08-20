package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrorNotVlessLink = errors.New("not a correct vless link")
)

// Vless is a vless proxy (vless://)
type Vless struct {
	Base
	UUID           string            `yaml:"uuid" json:"uuid"`
	Network        string            `yaml:"network,omitempty" json:"network,omitempty"`
	Flow           string            `yaml:"flow,omitempty" json:"flow,omitempty"`
	ServerName     string            `yaml:"servername,omitempty" json:"servername,omitempty"`
	TLS            bool              `yaml:"tls,omitempty" json:"tls,omitempty"`
	SkipCertVerify bool              `yaml:"skip-cert-verify,omitempty" json:"skip-cert-verify,omitempty"`
	SNI            string            `yaml:"sni,omitempty" json:"sni,omitempty"`
	ALPN           []string          `yaml:"alpn,omitempty" json:"alpn,omitempty"`
	WSOpts         *WSOptions        `yaml:"ws-opts,omitempty" json:"ws-opts,omitempty"`
	GrpcOpts       *GrpcOptions      `yaml:"grpc-opts,omitempty" json:"grpc-opts,omitempty"`
	HTTPOpts       *HTTPOptions      `yaml:"http-opts,omitempty" json:"http-opts,omitempty"`
	RealityOpts    *RealityOptions   `yaml:"reality-opts,omitempty" json:"reality-opts,omitempty"`
	Fingerprint    string            `yaml:"client-fingerprint,omitempty" json:"client-fingerprint,omitempty"`
}

type GrpcOptions struct {
	GrpcServiceName string `yaml:"grpc-service-name,omitempty" json:"grpc-service-name,omitempty"`
}

type RealityOptions struct {
	PublicKey string `yaml:"public-key,omitempty" json:"public-key,omitempty"`
	ShortID  string `yaml:"short-id,omitempty" json:"short-id,omitempty"`
}

func (v Vless) Identifier() string {
	return net.JoinHostPort(v.Server, strconv.Itoa(v.Port)) + v.UUID + v.Flow
}

func (v Vless) String() string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func (v Vless) ToClash() string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return "- " + string(data)
}

func (v Vless) ToSurge() string {
	return ""
}

func (v Vless) Clone() Proxy {
	return &v
}

func (v Vless) Link() string {
	query := url.Values{}
	if v.Network != "" {
		query.Set("type", v.Network)
	}
	if v.Flow != "" {
		query.Set("flow", v.Flow)
	}
	if v.TLS {
		query.Set("security", "tls")
	}
	if v.SNI != "" {
		query.Set("sni", v.SNI)
	}
	if v.SkipCertVerify {
		query.Set("allowInsecure", "1")
	}
	if v.Fingerprint != "" {
		query.Set("fp", v.Fingerprint)
	}
	if v.WSOpts != nil {
		query.Set("path", v.WSOpts.Path)
		if host, ok := v.WSOpts.Headers["Host"]; ok && host != "" {
			query.Set("host", host)
		}
	}
	if v.GrpcOpts != nil && v.GrpcOpts.GrpcServiceName != "" {
		query.Set("serviceName", v.GrpcOpts.GrpcServiceName)
	}
	if v.RealityOpts != nil {
		query.Set("security", "reality")
		query.Set("pbk", v.RealityOpts.PublicKey)
		query.Set("sid", v.RealityOpts.ShortID)
	}
	uri := url.URL{
		Scheme:   "vless",
		User:     url.User(v.UUID),
		Host:     net.JoinHostPort(v.Server, strconv.Itoa(v.Port)),
		RawQuery: query.Encode(),
		Fragment: v.Name,
	}
	return uri.String()
}

// ParseVlessLink parses a vless:// link
func ParseVlessLink(link string) (*Vless, error) {
	if !strings.HasPrefix(link, "vless://") {
		return nil, ErrorNotVlessLink
	}
	uri, err := url.Parse(link)
	if err != nil {
		return nil, ErrorNotVlessLink
	}
	server := uri.Hostname()
	if server == "" {
		return nil, ErrorNotVlessLink
	}
	port, _ := strconv.Atoi(uri.Port())
	if port == 0 {
		port = 443
	}
	uuid := uri.User.Username()
	if uuid == "" {
		return nil, ErrorNotVlessLink
	}
	query := uri.Query()
	v := &Vless{
		Base: Base{
			Name:   uri.Fragment,
			Server: server,
			Port:   port,
			Type:   "vless",
		},
		UUID:        uuid,
		Network:     query.Get("type"),
		Flow:        query.Get("flow"),
		Fingerprint: query.Get("fp"),
	}
	if network := v.Network; network == "" {
		v.Network = "tcp"
	}
	security := query.Get("security")
	if security == "tls" {
		v.TLS = true
	}
	v.SNI = query.Get("sni")
	v.ServerName = v.SNI
	if query.Get("allowInsecure") == "1" {
		v.SkipCertVerify = true
	}
	if alpn := query.Get("alpn"); alpn != "" {
		v.ALPN = strings.Split(alpn, ",")
	}
	if v.Network == "ws" {
		path := query.Get("path")
		if path == "" {
			path = "/"
		}
		headers := make(map[string]string)
		if host := query.Get("host"); host != "" {
			headers["Host"] = host
		}
		v.WSOpts = &WSOptions{Path: path, Headers: headers}
	}
	if v.Network == "grpc" {
		v.GrpcOpts = &GrpcOptions{GrpcServiceName: query.Get("serviceName")}
	}
	if security == "reality" {
		v.RealityOpts = &RealityOptions{
			PublicKey: query.Get("pbk"),
			ShortID:   query.Get("sid"),
		}
		v.SNI = query.Get("sni")
		v.ServerName = v.SNI
	}
	return v, nil
}

var (
	vlessPlainRe = regexp.MustCompile("vless://([A-Za-z0-9+/_&?=@:%.-])+")
)

func GrepVlessLinkFromString(text string) []string {
	results := make([]string, 0)
	if !strings.Contains(text, "vless://") {
		return results
	}
	texts := strings.Split(text, "vless://")
	for _, t := range texts {
		results = append(results, vlessPlainRe.FindAllString("vless://"+t, -1)...)
	}
	return results
}

var _ = fmt.Sprintf
