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
	ErrorNotHttpLink  = errors.New("not a correct http link")
	ErrorNotSocksLink = errors.New("not a correct socks link")
)

// Http is an HTTP/HTTPS proxy
type Http struct {
	Base
	Username       string `yaml:"username,omitempty" json:"username,omitempty"`
	Password       string `yaml:"password,omitempty" json:"password,omitempty"`
	TLS            bool   `yaml:"tls,omitempty" json:"tls,omitempty"`
	SkipCertVerify bool   `yaml:"skip-cert-verify,omitempty" json:"skip-cert-verify,omitempty"`
}

func (h Http) Identifier() string {
	return h.Type + net.JoinHostPort(h.Server, strconv.Itoa(h.Port)) + h.Username + h.Password
}

func (h Http) String() string {
	data, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return string(data)
}

func (h Http) ToClash() string {
	data, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return "- " + string(data)
}

func (h Http) ToSurge() string {
	if h.Username != "" {
		return fmt.Sprintf("%s = http, %s, %d, %s, %s", h.Name, h.Server, h.Port, h.Username, h.Password)
	}
	return fmt.Sprintf("%s = http, %s, %d", h.Name, h.Server, h.Port)
}

func (h Http) Clone() Proxy {
	return &h
}

func (h Http) Link() string {
	var user *url.Userinfo
	if h.Username != "" || h.Password != "" {
		user = url.UserPassword(h.Username, h.Password)
	}
	scheme := "http"
	if h.TLS {
		scheme = "https"
	}
	uri := url.URL{
		Scheme: scheme,
		User:   user,
		Host:   net.JoinHostPort(h.Server, strconv.Itoa(h.Port)),
	}
	return uri.String()
}

// ParseHttpLink parses an http:// or https:// proxy link
func ParseHttpLink(link string) (*Http, error) {
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		return nil, ErrorNotHttpLink
	}
	uri, err := url.Parse(link)
	if err != nil {
		return nil, ErrorNotHttpLink
	}
	server := uri.Hostname()
	if server == "" {
		return nil, ErrorNotHttpLink
	}
	port, _ := strconv.Atoi(uri.Port())
	if port == 0 {
		if uri.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	username := uri.User.Username()
	password, _ := uri.User.Password()
	return &Http{
		Base: Base{
			Name:   uri.Fragment,
			Server: server,
			Port:   port,
			Type:   "http",
		},
		Username:       username,
		Password:       password,
		TLS:            uri.Scheme == "https",
		SkipCertVerify: true,
	}, nil
}

// Socks5 is a SOCKS proxy (socks4/socks5)
type Socks5 struct {
	Base
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	UDP      bool   `yaml:"udp,omitempty" json:"udp,omitempty"`
}

func (s Socks5) Identifier() string {
	return s.Type + net.JoinHostPort(s.Server, strconv.Itoa(s.Port)) + s.Username + s.Password
}

func (s Socks5) String() string {
	data, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(data)
}

func (s Socks5) ToClash() string {
	data, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return "- " + string(data)
}

func (s Socks5) ToSurge() string {
	if s.Username != "" {
		return fmt.Sprintf("%s = socks5, %s, %d, %s, %s", s.Name, s.Server, s.Port, s.Username, s.Password)
	}
	return fmt.Sprintf("%s = socks5, %s, %d", s.Name, s.Server, s.Port)
}

func (s Socks5) Clone() Proxy {
	return &s
}

func (s Socks5) Link() string {
	var user *url.Userinfo
	if s.Username != "" || s.Password != "" {
		user = url.UserPassword(s.Username, s.Password)
	}
	uri := url.URL{
		Scheme: s.Type,
		User:   user,
		Host:   net.JoinHostPort(s.Server, strconv.Itoa(s.Port)),
	}
	return uri.String()
}

// ParseSocksLink parses a socks5:// or socks4:// proxy link
func ParseSocksLink(link string) (*Socks5, error) {
	if !strings.HasPrefix(link, "socks5://") && !strings.HasPrefix(link, "socks4://") && !strings.HasPrefix(link, "socks4a://") && !strings.HasPrefix(link, "socks5h://") {
		return nil, ErrorNotSocksLink
	}
	uri, err := url.Parse(link)
	if err != nil {
		return nil, ErrorNotSocksLink
	}
	server := uri.Hostname()
	if server == "" {
		return nil, ErrorNotSocksLink
	}
	port, _ := strconv.Atoi(uri.Port())
	if port == 0 {
		port = 1080
	}
	username := uri.User.Username()
	password, _ := uri.User.Password()
	// clash only supports socks5, unify socks4/socks4a/socks5h into socks5
	proxyType := "socks5"
	return &Socks5{
		Base: Base{
			Name:   uri.Fragment,
			Server: server,
			Port:   port,
			Type:   proxyType,
		},
		Username: username,
		Password: password,
		UDP:      true,
	}, nil
}

var (
	socksPlainRe = regexp.MustCompile("socks[45][a-h]?://([A-Za-z0-9+/_&?=@:%.-])+")
)

// GrepSocksLinkFromString extracts socks proxy links from text
func GrepSocksLinkFromString(text string) []string {
	results := make([]string, 0)
	if !strings.Contains(text, "socks5://") && !strings.Contains(text, "socks4://") {
		return results
	}
	results = append(results, socksPlainRe.FindAllString(text, -1)...)
	return results
}
