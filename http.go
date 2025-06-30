package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func fwdHttp(outboundClient *http.Client, targetAddr string, w http.ResponseWriter, r *http.Request) error {
	var proxyError error

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL, _ = url.Parse(targetAddr + req.URL.RequestURI())
			req.Host = req.URL.Host
		},
		Transport: outboundClient.Transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "error proxying request", http.StatusBadGateway)

			proxyError = err
		},
	}

	proxy.ServeHTTP(w, r)

	return proxyError
}
