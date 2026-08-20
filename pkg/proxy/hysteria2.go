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
	ErrorNotHysteria2Link = errors.New("not a correct hysteria2 link")
)

// Hysteria2 is a hysteria2 proxy (hy2://)
type Hysteria2 struct {
	Base
	Password       string `yaml:"password" json:"password"`
	Network        string `yaml:"network,omitempty" json:"network,omitempty"`
	Obfs           string `yaml:"obfs,omitempty" json:"obfs,omitempty"`
	ObfsPassword   string `yaml:"obfs-password,omitempty" json:"obfs-password,omitempty"`
	SNI            string `yaml:"sni,omitempty" json:"sni,omitempty"`
	SkipCertVerify bool   `yaml:"skip-cert-verify,omitempty" json:"skip-cert-verify,omitempty"`
	ALPN           []string `yaml:"alpn,omitempty" json:"alpn,omitempty"`
	Up             int    `yaml:"up,omitempty" json:"up,omitempty"`
	Down           int    `yaml:"down,omitempty" json:"down,omitempty"`
}

func (h Hysteria2) Identifier() string {
	return net.JoinHostPort(h.Server, strconv.Itoa(h.Port)) + h.Password
}

func (h Hysteria2) String() string {
	data, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return string(data)
}

func (h Hysteria2) ToClash() string {
	data, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return "- " + string(data)
}

func (h Hysteria2) ToSurge() string {
	return ""
}

func (h Hysteria2) Clone() Proxy {
	return &h
}

func (h Hysteria2) Link() string {
	query := url.Values{}
	if h.SNI != "" {
		query.Set("sni", h.SNI)
	}
	if h.Obfs != "" {
		query.Set("obfs", h.Obfs)
		query.Set("obfs-password", h.ObfsPassword)
	}
	if h.SkipCertVerify {
		query.Set("insecure", "1")
	}
	if h.Network != "" {
		query.Set("network", h.Network)
	}
	uri := url.URL{
		Scheme:   "hy2",
		User:     url.UserPassword(h.Password, ""),
		Host:     net.JoinHostPort(h.Server, strconv.Itoa(h.Port)),
		RawQuery: query.Encode(),
		Fragment: h.Name,
	}
	return uri.String()
}

// ParseHysteria2Link parses a hy2:// or hysteria2:// link
func ParseHysteria2Link(link string) (*Hysteria2, error) {
	if !strings.HasPrefix(link, "hy2://") && !strings.HasPrefix(link, "hysteria2://") {
		return nil, ErrorNotHysteria2Link
	}
	uri, err := url.Parse(link)
	if err != nil {
		return nil, ErrorNotHysteria2Link
	}
	server := uri.Hostname()
	if server == "" {
		return nil, ErrorNotHysteria2Link
	}
	port, _ := strconv.Atoi(uri.Port())
	if port == 0 {
		port = 443
	}
	password := uri.User.Username()
	password, _ = url.QueryUnescape(password)
	query := uri.Query()
	h := &Hysteria2{
		Base: Base{
			Name:   uri.Fragment,
			Server: server,
			Port:   port,
			Type:   "hysteria2",
		},
		Password:       password,
		SNI:            query.Get("sni"),
		Obfs:           query.Get("obfs"),
		ObfsPassword:   query.Get("obfs-password"),
		Network:        query.Get("network"),
		SkipCertVerify: query.Get("insecure") == "1",
	}
	if alpn := query.Get("alpn"); alpn != "" {
		h.ALPN = strings.Split(alpn, ",")
	}
	return h, nil
}

var (
	hy2PlainRe = regexp.MustCompile("hys(?:teria2)?://([A-Za-z0-9+/_&?=@:%.-])+")
)

func GrepHysteria2LinkFromString(text string) []string {
	results := make([]string, 0)
	if !strings.Contains(text, "hy2://") && !strings.Contains(text, "hysteria2://") {
		return results
	}
	if strings.Contains(text, "hy2://") {
		texts := strings.Split(text, "hy2://")
		for _, t := range texts {
			results = append(results, hy2PlainRe.FindAllString("hy2://"+t, -1)...)
		}
	}
	if strings.Contains(text, "hysteria2://") {
		texts := strings.Split(text, "hysteria2://")
		for _, t := range texts {
			results = append(results, hy2PlainRe.FindAllString("hysteria2://"+t, -1)...)
		}
	}
	return results
}

var _ = fmt.Sprintf
