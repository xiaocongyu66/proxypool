package getter

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ssrlive/proxypool/log"
	"github.com/ssrlive/proxypool/pkg/proxy"
	"github.com/ssrlive/proxypool/pkg/tool"
)

func init() {
	Register("github-proxylist", NewGitHubProxyList)
}

// GitHubProxyList fetches a proxy list file from a GitHub repo, but only if the
// repo has been updated within the configured freshness window (default 7 days).
// Stale repos are skipped to avoid crawling abandoned sources.
type GitHubProxyList struct {
	Url             string
	Repo            string
	MaxAgeDays      int
	Token           string
	DefaultProtocol string
}

func NewGitHubProxyList(options tool.Options) (getter Getter, err error) {
	urlInterface, found := options["url"]
	if !found {
		return nil, ErrorUrlNotFound
	}
	url, err := AssertTypeStringNotNull(urlInterface)
	if err != nil {
		return nil, err
	}
	repoInterface, _ := options["repo"]
	repo, _ := repoInterface.(string)
	maxAge := 7
	if v, ok := options["max-age-days"]; ok {
		switch n := v.(type) {
		case int:
			maxAge = n
		case float64:
			maxAge = int(n)
		}
	}
	if maxAge <= 0 {
		maxAge = 7
	}
	token, _ := options["token"].(string)
	defaultProtocol, _ := options["default-protocol"].(string)
	return &GitHubProxyList{
		Url:             url,
		Repo:            repo,
		MaxAgeDays:      maxAge,
		Token:           token,
		DefaultProtocol: defaultProtocol,
	}, nil
}

func (g *GitHubProxyList) Get() proxy.ProxyList {
	result := make(proxy.ProxyList, 0)
	if g.Repo != "" {
		if !g.isRepoFresh() {
			log.Warnln("[github-proxylist] repo %s not updated within %d days, skip crawling", g.Repo, g.MaxAgeDays)
			return result
		}
	}
	resp, err := tool.GetHttpClient().Get(g.Url)
	if err != nil {
		log.Errorln("[github-proxylist] fetch %s error: %s", g.Url, err.Error())
		return result
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := g.parseLine(line)
		if p != nil {
			result = append(result, p)
		}
	}
	return result
}

func (g *GitHubProxyList) parseLine(line string) proxy.Proxy {
	line = strings.TrimSpace(line)
	// lines with explicit scheme: socks5://, http://, https://, socks4://
	if strings.Contains(line, "://") {
		if p, err := proxy.ParseProxyFromLink(line); err == nil && p != nil {
			return p
		}
		return nil
	}
	// bare ip:port, use configured default protocol
	if g.DefaultProtocol != "" {
		scheme := g.DefaultProtocol
		if scheme != "socks5" && scheme != "socks4" && scheme != "http" && scheme != "https" {
			scheme = "socks5"
		}
		if p, err := proxy.ParseProxyFromLink(scheme + "://" + line); err == nil && p != nil {
			return p
		}
	}
	return nil
}

type ghCommit struct {
	Commit struct {
		Author struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// isRepoFresh checks whether the GitHub repo's latest commit is within MaxAgeDays.
func (g *GitHubProxyList) isRepoFresh() bool {
	apiUrl := "https://api.github.com/repos/" + g.Repo + "/commits?per_page=1"
	req, err := http.NewRequest(http.MethodGet, apiUrl, nil)
	if err != nil {
		return true // fail-open: proceed to crawl when check unavailable
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", tool.UserAgent)
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warnln("[github-proxylist] update check failed for %s: %s, fail-open", g.Repo, err.Error())
		return true
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warnln("[github-proxylist] update check for %s returned status %d, fail-open", g.Repo, resp.StatusCode)
		return true
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return true
	}
	var commits []ghCommit
	if err := json.Unmarshal(body, &commits); err != nil || len(commits) == 0 {
		return true
	}
	dateStr := commits[0].Commit.Author.Date
	commitTime, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return true
	}
	age := time.Since(commitTime)
	fresh := age <= time.Duration(g.MaxAgeDays)*24*time.Hour
	log.Infoln("[github-proxylist] repo %s last commit %s ago, fresh=%v (threshold %d days)", g.Repo, age.Round(time.Hour), fresh, g.MaxAgeDays)
	return fresh
}

func (g *GitHubProxyList) Get2ChanWG(pc chan proxy.Proxy, wg *sync.WaitGroup) {
	defer wg.Done()
	nodes := g.Get()
	log.Infoln("STATISTIC: GitHubProxyList\tcount=%d\turl=%s", len(nodes), g.Url)
	for _, node := range nodes {
		pc <- node
	}
}
