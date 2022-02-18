package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type Proxy struct {
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("[%s] %s %s %s\n", req.Method, req.Host, req.RequestURI, req.RemoteAddr)

	transport := http.DefaultTransport

	outReq := new(http.Request)
	*outReq = *req

	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		prior, ok := outReq.Header["X-Forwarded-For"]
		if ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}

	res, err := transport.RoundTrip(outReq)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	for key, value := range res.Header {
		for _, v := range value {
			w.Header().Add(key, v)
		}
	}

	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	res.Body.Close()
}

func main() {
	addr := ":7890"
	log.Info("Listen on " + addr)
	go selfTest("localhost" + addr)
	http.Handle("/", &Proxy{})
	err := http.ListenAndServe(addr, nil)
	abortErr(err)

}

func abortErr(err error) {
	if err != nil {
		log.Fatal(err)
		log.Exit(1)
	}
}

func selfTest(addr string) {
	time.Sleep(time.Second * 3)
	log.Info("Started self-test")
	// do a get request over proxy
	proxyUrl, err := url.Parse("http://" + addr)
	abortErr(err)
	var transport = &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	log.Info("Transport created")
	client := &http.Client{Transport: transport, Timeout: time.Second * 5}
	req, err := http.NewRequest("GET", "http://www.google.com", nil)
	abortErr(err)
	log.Info("Doing GET request")
	resp, err := client.Do(req)
	log.Info("Got response")
	abortErr(err)

	if err != nil {
		abortErr(errors.New("Failed to pass self-test: " + err.Error()))
	} else if resp.StatusCode != 200 {
		respBody, err := ioutil.ReadAll(resp.Body)
		abortErr(err)
		abortErr(errors.New("Failed to pass self-test, status: " + resp.Status + " response: " + string(respBody)))
	} else {
		log.Info("Passed self-test")
	}
}
