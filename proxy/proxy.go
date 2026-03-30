package proxy

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

type clientKey struct {
	host string
	mtls bool
}

type Proxy struct {
	clients    sync.Map
	timeout    time.Duration
	mtlsConfig *tls.Config // nil when mTLS is not configured
}

func NewProxy(timeout time.Duration) *Proxy {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Proxy{timeout: timeout}
}

// NewProxyWithMTLS creates a Proxy that can optionally use mTLS for upstream
// requests. mtlsCfg may be nil, in which case mTLS calls fall back to plain TLS.
func NewProxyWithMTLS(timeout time.Duration, mtlsCfg *tls.Config) *Proxy {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Proxy{timeout: timeout, mtlsConfig: mtlsCfg}
}

func (p *Proxy) getClient(key clientKey) *http.Client {
	if v, ok := p.clients.Load(key); ok {
		return v.(*http.Client)
	}
	transport := &http.Transport{
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	if key.mtls && p.mtlsConfig != nil {
		transport.TLSClientConfig = p.mtlsConfig.Clone()
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   p.timeout,
	}
	actual, _ := p.clients.LoadOrStore(key, client)
	return actual.(*http.Client)
}

func (p *Proxy) Do(req *http.Request) (*http.Response, error) {
	return p.DoMTLS(req, false)
}

// DoMTLS executes the request, optionally presenting a client certificate when
// mtls is true and the proxy was configured with an mTLS config.
func (p *Proxy) DoMTLS(req *http.Request, mtls bool) (*http.Response, error) {
	key := clientKey{host: req.URL.Host, mtls: mtls && p.mtlsConfig != nil}
	client := p.getClient(key)
	return client.Do(req)
}
