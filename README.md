simpleproxy
=====================
simple proxy is a really simple reverse proxy - mostly for development purposes.

installation
=====================
   go get github.com/ybrs/simpleproxy.git

or

   git clone
   go build

usage
=====================

    simpleproxy -listen 0.0.0.0:9001 -target http://localhost:8000