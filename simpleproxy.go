package simpleproxy

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
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
