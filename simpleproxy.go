package vproxy

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var badGatewayMessage = `<html>
<body>
<h1>502 Bad Gateway</h1>
<p>Can't connect to upstream server, please try again later.</p>
</body>
</html>`

// proxyTransport is a simple http.RoundTripper implementation which returns a
// 503 on any error making a request to the upstream (backend) service
type proxyTransport struct {
}

func (t *proxyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(request)

	if err != nil {
		resp := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       ioutil.NopCloser(bytes.NewBufferString(badGatewayMessage)),
		}

		resp.StatusCode = 503
		resp.Status = "Can't connect to upstream server"
		log.Println("error ", err)
		return resp, nil
	}

	return response, err
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
		Transport: &proxyTransport{},
	}
}
