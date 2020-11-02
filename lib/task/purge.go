package task

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/mudkipme/timburr/utils"
	log "github.com/sirupsen/logrus"
)

// PurgeExecutor purges the front-end cache by URLs
type PurgeExecutor struct {
	expiry   time.Duration
	entries  []utils.PurgeEntryConfig
	client   *http.Client
	cfAPI    *cloudflare.API
	cfZoneID string
}

type requestOptions struct {
	method  string
	url     string
	headers map[string]string
}

// DefaultPurgeExecutor creates a purge executor based on config.yml
func DefaultPurgeExecutor() *PurgeExecutor {
	var cfAPI *cloudflare.API
	var err error
	if utils.Config.Purge.CFToken != "" {
		cfAPI, err = cloudflare.NewWithAPIToken(utils.Config.Purge.CFToken)
		if err != nil {
			log.WithError(err).Error("cloudflare api invalid")
		}
	}
	return NewPurgeExecutor(
		time.Millisecond*time.Duration(utils.Config.Purge.Expiry),
		utils.Config.Purge.Entries,
		cfAPI,
		utils.Config.Purge.CFZoneID,
	)
}

// NewPurgeExecutor creates a new purge executor
func NewPurgeExecutor(expiry time.Duration, entries []utils.PurgeEntryConfig, cfAPI *cloudflare.API, cfZoneID string) *PurgeExecutor {
	return &PurgeExecutor{
		expiry:  expiry,
		entries: entries,
		client: &http.Client{
			Timeout: time.Second * 2,
		},
		cfAPI:    cfAPI,
		cfZoneID: cfZoneID,
	}
}

// Execute sends the corresponding PURGE requests from a kafka message
func (t *PurgeExecutor) Execute(message []byte) error {
	type purgeData struct {
		Meta struct {
			URI  string    `json:"uri"`
			Date time.Time `json:"dt"`
		} `json:"meta"`
	}
	var msg purgeData
	err := json.Unmarshal(message, &msg)
	if err != nil {
		return err
	}

	// skip expired message
	if time.Now().After(msg.Meta.Date.Add(t.expiry)) {
		log.WithField("url", msg.Meta.URI).Info("skip purge")
		return nil
	}

	t.handlePurge(msg.Meta.URI)
	return nil
}

func (t *PurgeExecutor) handlePurge(item string) {
	ros := []requestOptions{}
	u, err := url.Parse(item)
	if err != nil {
		return
	}
	queries := strings.Split(u.RawQuery, "&")
	lastQuery := ""
	if len(queries) > 0 {
		lastQuery = queries[len(queries)-1]
	}
	pathComponents := strings.Split(u.RawPath, "/")
	firstPath := ""
	if len(pathComponents) > 1 {
		firstPath = pathComponents[1]
	}

	for _, entry := range t.entries {
		if entry.Host != u.Host {
			continue
		}
		variants := make(map[string]bool)
		variants[""] = true
		for _, variant := range entry.Variants {
			variants[variant] = true
		}

		for _, uri := range entry.URIs {
			for variant := range variants {
				if variant != "" && ((lastQuery != "" && variants[lastQuery]) ||
					(firstPath != "" && variants[firstPath])) {
					continue
				}
				ros = append(ros, requestOptions{
					method:  entry.Method,
					url:     strings.ReplaceAll(strings.ReplaceAll(uri, "#url#", u.RequestURI()), "#variants#", variant),
					headers: entry.Headers,
				})

				if firstPath == "wiki" && variant != "" {
					ros = append(ros, requestOptions{
						method:  entry.Method,
						url:     strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(uri, "#url#", u.RequestURI()), "#variants#", ""), "/wiki/", "/"+variant+"/"),
						headers: entry.Headers,
					})
				}
			}
		}
	}

	ros = uniq(ros)
	ch := make(chan bool)
	for _, ro := range ros {
		go t.doRequest(ro.method, ro.url, ro.headers, ch)
	}
	for range ros {
		<-ch
	}
}

func (t *PurgeExecutor) doRequest(method, url string, headers map[string]string, ch chan bool) {
	if strings.ToLower(method) == "cloudflare" {
		t.doCloudFlarePurge(url, ch)
		return
	}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.WithError(err).WithField("url", url).Warn("failed to send purge request")
		ch <- false
		return
	}
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}
	response, err := t.client.Do(req)
	if err != nil {
		log.WithError(err).WithField("url", url).Warn("failed to send purge request")
		ch <- false
		return
	}
	response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		log.WithField("url", url).Info("purge success")
	} else if response.StatusCode != http.StatusNotFound && response.StatusCode >= 300 {
		log.WithField("statusCode", response.StatusCode).WithField("url", url).Warn("failed to send purge request")
	}
	ch <- true
}

func (t *PurgeExecutor) doCloudFlarePurge(url string, ch chan bool) {
	if t.cfAPI == nil {
		log.WithField("url", url).Warn("failed to purge cloudflare cache")
		ch <- false
		return
	}
	resp, err := t.cfAPI.PurgeCache(t.cfZoneID, cloudflare.PurgeCacheRequest{
		Files: []string{url},
	})
	if err != nil {
		log.WithError(err).WithField("url", url).Warn("failed to purge cloudflare cache")
		ch <- false
		return
	}
	log.WithField("url", url).WithField("response", resp).Info("purge success")
	ch <- true
}

func uniq(input []requestOptions) (res []requestOptions) {
	res = make([]requestOptions, 0, len(input))
	seen := make(map[string]bool)
	for _, val := range input {
		if _, ok := seen[val.url]; !ok {
			seen[val.url] = true
			res = append(res, val)
		}
	}
	return
}
