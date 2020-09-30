package simpleproxy

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var BadGatewayMessage = `<html>
<body>
<h1>502 Bad Gateway</h1>
<p>Can't connect to upstream server, please try again later.</p>
</body>
</html>`

type ProxyTransport struct {
}

func (t *ProxyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(request)

	if err != nil {
		resp := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       ioutil.NopCloser(bytes.NewBufferString(BadGatewayMessage)),
		}

		resp.StatusCode = 503
		resp.Status = "Can't connect to upstream server"
		log.Println("error ", err)
		return resp, nil
	}

	return response, err
}

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
		Transport: &ProxyTransport{},
	}
}
