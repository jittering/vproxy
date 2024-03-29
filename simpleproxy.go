package vproxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var badGatewayMessage = `<html>
<body>
<h1>503 Service Unavailable</h1>
<p>Can't connect to upstream server (%s &mdash;&gt; %s), please try again later.</p>
</body>
</html>`

// proxyTransport is a simple http.RoundTripper implementation which returns a
// 503 on any error making a request to the upstream (backend) service
type proxyTransport struct {
	transport *http.Transport
	errMsg    string
}

func (t *proxyTransport) RoundTrip(request *http.Request) (*http.Response, error) {

	var (
		response *http.Response
		err      error
	)

	operation := func() error {
		response, err = t.transport.RoundTrip(request)
		// Handle Retry-After here, if you wish...
		// If err is nil, no retry will occur
		return err
	}

	be := backoff.NewExponentialBackOff()
	be.MaxElapsedTime = time.Second * 30
	err = backoff.Retry(operation, be)
	if err != nil {
		resp := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       ioutil.NopCloser(bytes.NewBufferString(t.errMsg)),
		}

		resp.StatusCode = 503
		resp.Status = "Can't connect to upstream server"
		log.Println("proxy: error fetching from upstream:", err)
		return resp, nil
	}

	return response, err
}

func createProxyTransport(targetURL url.URL, vhost string) *proxyTransport {
	t := &proxyTransport{errMsg: fmt.Sprintf(badGatewayMessage, targetURL.String(), vhost)}
	t.transport = http.DefaultTransport.(*http.Transport).Clone()
	t.transport.MaxConnsPerHost = 0 // unlim
	t.transport.MaxIdleConns = 800
	t.transport.MaxIdleConnsPerHost = 8
	return t
}

// CreateProxy with custom http.RoundTripper impl. Sets proper host headers
// using given vhost name.
func CreateProxy(targetURL url.URL, vhost string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			p, q := r.URL.Path, r.URL.RawQuery
			*r.URL = targetURL
			r.URL.Path, r.URL.RawQuery = p, q
			if vhost != "" {
				r.Host = vhost
				r.Header.Add("X-Forwarded-Host", vhost)
			} else {
				r.Host = targetURL.Host
			}
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			r.Header.Add("X-Forwarded-Proto", scheme)
		},
		Transport: createProxyTransport(targetURL, vhost),
	}
}
