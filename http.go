package simpleproxy

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type LoggedMux struct {
	*http.ServeMux
}

func NewLoggedMux() *LoggedMux {
	var mux = &LoggedMux{}
	mux.ServeMux = http.NewServeMux()
	return mux
}

type LogRecord struct {
	http.ResponseWriter
	status        int
	responseBytes int64
}

func (r *LogRecord) Write(p []byte) (int, error) {
	written, err := r.ResponseWriter.Write(p)
	r.responseBytes += int64(written)
	return written, err
}

func (r *LogRecord) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (mux *LoggedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	record := &LogRecord{
		ResponseWriter: w,
	}

	// ignore port num, if any
	s := strings.Split(r.Host, ":")
	host := s[0]

	startTime := time.Now()
	mux.ServeMux.ServeHTTP(record, r)
	finishTime := time.Now()
	elapsedTime := finishTime.Sub(startTime)

	log.Printf("%s [%s] %s [ %d ] %s %d %s", r.RemoteAddr, host, r.Method, record.status, r.URL, r.ContentLength, elapsedTime)
}
