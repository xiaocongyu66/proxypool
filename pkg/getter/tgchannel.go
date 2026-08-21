package getter

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/ssrlive/proxypool/log"

	"github.com/gocolly/colly"
	"github.com/ssrlive/proxypool/pkg/proxy"
	"github.com/ssrlive/proxypool/pkg/tool"
)

func init() {
	Register("tgchannel", NewTGChannelGetter)
}

type TGChannelGetter struct {
	c         *colly.Collector
	NumNeeded int
	results   []string
	Url       string
	apiUrl    string
}

func NewTGChannelGetter(options tool.Options) (getter Getter, err error) {
	num, found := options["num"]
	t := 200
	switch num := num.(type) {
	case int:
		t = num
	case float64:
		t = int(num)
	}

	if !found || t <= 0 {
		t = 200
	}
	urlInterface, found := options["channel"]
	if found {
		url, err := AssertTypeStringNotNull(urlInterface)
		if err != nil {
			return nil, err
		}
		return &TGChannelGetter{
			c:         tool.GetColly(),
			NumNeeded: t,
			Url:       "https://t.me/s/" + url,
			apiUrl:    "https://tg.i-c-a.su/rss/" + url,
		}, nil
	}
	return nil, ErrorUrlNotFound
}

func (g *TGChannelGetter) Get() proxy.ProxyList {
	result := make(proxy.ProxyList, 0)
	g.results = make([]string, 0)
	// 找到所有的文字消息
	g.c.OnHTML("div.tgme_widget_message_text", func(e *colly.HTMLElement) {
		g.results = append(g.results, GrepLinksFromString(e.Text)...)
		// 解析 Telegram SOCKS 代理链接: t.me/socks?server=IP&port=PORT 或 tg://socks?...
		for _, link := range parseTelegramProxyLinks(e.Text) {
			if p, err := proxy.ParseProxyFromLink(link); err == nil && p != nil {
				result = append(result, p)
			}
		}
		// 抓取到http链接，有可能是订阅链接或其他链接，无论如何试一下
		subUrls := urlRe.FindAllString(e.Text, -1)
		for _, u := range subUrls {
			result = append(result, (&Subscribe{Url: u}).Get()...)
		}
	})

	// 找到之前消息页面的链接，加入访问队列
	g.c.OnHTML("link[rel=prev]", func(e *colly.HTMLElement) {
		if len(g.results) < g.NumNeeded {
			_ = e.Request.Visit(e.Attr("href"))
		}
	})

	g.results = make([]string, 0)
	err := g.c.Visit(g.Url)
	if err != nil {
		_ = fmt.Errorf("%s", err.Error())
	}
	result = append(result, StringArray2ProxyArray(g.results)...)

	// 获取文件(api需要维护)
	resp, err := tool.GetHttpClient().Get(g.apiUrl)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	items := strings.Split(string(body), "\n")
	for _, s := range items {
		if strings.Contains(s, "enclosure url") { // get to xml node
			elements := strings.Split(s, "\"")
			for _, e := range elements {
				if strings.Contains(e, "https://") {
					// Webfuzz的可能性比较大，也有可能是订阅链接，为了不拖慢运行速度不写了
					result = append(result, (&WebFuzz{Url: e}).Get()...)
				}
			}
		}
	}
	return result
}

func (g *TGChannelGetter) Get2ChanWG(pc chan proxy.Proxy, wg *sync.WaitGroup) {
	defer wg.Done()
	nodes := g.Get()
	log.Infoln("STATISTIC: TGChannel\tcount=%d\turl=%s", len(nodes), g.Url)
	for _, node := range nodes {
		pc <- node
	}
}

var tgProxyLinkRe = regexp.MustCompile(`(?:https?://t\.me/socks\?|tg://socks\?|https?://t\.me/proxy\?|tg://proxy\?)([^"'<>\s]+)`)

// parseTelegramProxyLinks extracts Telegram proxy links from text.
// Supports: t.me/socks?server=IP&port=PORT, tg://socks?server=IP&port=PORT
// Converts them to socks5://IP:PORT format.
func parseTelegramProxyLinks(text string) []string {
	results := make([]string, 0)
	matches := tgProxyLinkRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		rawQuery := m[1]
		// Extract server and port from query params
		vals, err := url.ParseQuery(rawQuery)
		if err != nil {
			continue
		}
		server := vals.Get("server")
		port := vals.Get("port")
		if server == "" || port == "" {
			continue
		}
		user := vals.Get("user")
		pass := vals.Get("pass")
		// Determine protocol: socks or proxy (proxy = MTProto, skip for now)
		if strings.Contains(m[0], "socks") {
			if user != "" && pass != "" {
				results = append(results, fmt.Sprintf("socks5://%s:%s@%s:%s", user, pass, server, port))
			} else {
				results = append(results, fmt.Sprintf("socks5://%s:%s", server, port))
			}
		}
	}
	return results
}
