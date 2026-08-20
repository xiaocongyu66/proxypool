package healthcheck

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/Dreamacro/clash/adapter"
	C "github.com/Dreamacro/clash/constant"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

// contextWithTimeout creates a context with the given timeout.
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// parseClashProxy converts a proxy.Proxy to a clash C.Proxy for health checking.
func parseClashProxy(pmap map[string]interface{}, p proxy.Proxy) (C.Proxy, error) {
	if proxy.GoodNodeThatClashUnsupported(p) {
		return nil, errUnsupportedNode
	}
	return adapter.ParseProxy(pmap)
}

// newProxyTransport creates an http.Transport that routes through the given clash proxy.
func newProxyTransport(clashProxy C.Proxy, addr C.Metadata, timeout time.Duration) *http.Transport {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_ = cancel
	return &http.Transport{
		Dial: func(string, string) (net.Conn, error) {
			return clashProxy.DialContext(ctx, &addr)
		},
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

var errUnsupportedNode = &unsupportedNodeError{}

type unsupportedNodeError struct{}

func (e *unsupportedNodeError) Error() string { return "unsupported node type for clash" }

// parseProxyMap converts a proxy.Proxy to a map suitable for adapter.ParseProxy.
func parseProxyMap(p proxy.Proxy) (map[string]interface{}, error) {
	pmap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(p.String()), &pmap); err != nil {
		return nil, err
	}
	portVal, ok := pmap["port"].(float64)
	if !ok {
		return nil, errUnsupportedNode
	}
	pmap["port"] = int(portVal)
	if p.TypeName() == "vmess" {
		if aid, ok := pmap["alterId"].(float64); ok {
			pmap["alterId"] = int(aid)
		}
	}
	return pmap, nil
}
