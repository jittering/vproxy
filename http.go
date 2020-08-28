package main

import (
	"log"
	"net/http"
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

	startTime := time.Now()
	mux.ServeMux.ServeHTTP(record, r)
	finishTime := time.Now()
	elapsedTime := finishTime.Sub(startTime)
	log.Println(r.RemoteAddr, " ", r.Method, "[", record.status, "]", r.URL, r.ContentLength, elapsedTime)
}
