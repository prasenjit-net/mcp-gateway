package proxy

import (
	"net/http"
	"sync"
	"time"
)

type Proxy struct {
	clients sync.Map
	timeout time.Duration
}

func NewProxy(timeout time.Duration) *Proxy {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Proxy{timeout: timeout}
}

func (p *Proxy) getClient(host string) *http.Client {
	if v, ok := p.clients.Load(host); ok {
		return v.(*http.Client)
	}
	transport := &http.Transport{
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   p.timeout,
	}
	actual, _ := p.clients.LoadOrStore(host, client)
	return actual.(*http.Client)
}

func (p *Proxy) Do(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	client := p.getClient(host)
	return client.Do(req)
}
