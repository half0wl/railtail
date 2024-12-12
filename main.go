package main

import (
	"cmp"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"
	"tailscale.com/tsnet"
)

var (
	tsHostname = flag.String("ts-hostname", "", "hostname to use for tailscale (or set env: TS_HOSTNAME)")
	listenPort = flag.String("listen-port", "", "port to listen on (or set env: LISTEN_PORT)")
	targetAddr = flag.String("target-addr", "", "address:port of a tailscale node to send traffic to (or set env: TARGET_ADDR)")
	tsAuthKey  = cmp.Or(os.Getenv("TS_AUTH_KEY"), "")
)

func handleTcpConn(logger *zap.SugaredLogger, lstConn net.Conn, ts *tsnet.Server, target string) {
	defer lstConn.Close()

	logger.Infof("[tcp] fwd: %s -> %s -> %s", lstConn.LocalAddr(), lstConn.RemoteAddr(), target)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsConn, err := ts.Dial(ctx, "tcp", target)
	if err != nil {
		logger.Errorf("ts dial failed: %v", err)
		return
	}

	lstChan, tsChan := make(chan int64), make(chan int64)
	go func() {
		defer tsConn.Close()
		n, _ := io.Copy(tsConn, lstConn)
		lstChan <- int64(n)
	}()
	go func() {
		defer lstConn.Close()
		n, _ := io.Copy(lstConn, tsConn)
		tsChan <- int64(n)
	}()
	<-lstChan
	<-tsChan
}

func handleHttpConn(logger *zap.SugaredLogger, outboundClient *http.Client, targetAddr string, w http.ResponseWriter, r *http.Request) {
	targetUri := fmt.Sprintf("%s%s", targetAddr, r.URL.Path)

	logger.Infof("[http] %s %s", r.Method, targetUri)

	outReq, err := http.NewRequest(r.Method, targetUri, r.Body)
	if err != nil {
		logger.Errorf("error creating request: %v", err)
		http.Error(w, "error creating request", http.StatusInternalServerError)
		return
	}

	// Copy headers: in -> out
	for name, values := range r.Header {
		for _, value := range values {
			outReq.Header.Add(name, value)
		}
	}

	resp, err := outboundClient.Do(outReq)
	if err != nil {
		logger.Errorf("error sending request: %v", err)
		http.Error(w, "error sending request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers: out (resp) -> in
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	zapLgr, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	logger := zapLgr.Sugar()
	defer logger.Sync()

	flag.Parse()

	if tsAuthKey == "" {
		logger.Fatal("env TS_AUTH_KEY is required")
	}

	// Grab config from env if not provided in args. Args take precedence.
	if *tsHostname == "" {
		*tsHostname = os.Getenv("TS_HOSTNAME")
	}
	if *tsHostname == "" {
		logger.Fatal("ts-hostname is required (set TS_HOSTNAME in env or use -ts-hostname)")
	}

	if *listenPort == "" {
		*listenPort = os.Getenv("LISTEN_PORT")
	}
	if *listenPort == "" {
		logger.Fatal("listen-port is required (set LISTEN_PORT in env or use -listen-port)")
	}

	if *targetAddr == "" {
		*targetAddr = os.Getenv("TARGET_ADDR")
	}
	if *targetAddr == "" {
		logger.Fatal("target-addr is required (set TARGET_ADDR in env or use -target-addr)")
	}

	ts := &tsnet.Server{
		Hostname:     *tsHostname,
		AuthKey:      tsAuthKey,
		RunWebClient: false,
		Ephemeral:    false,
		Dir:          "/tmp/railtail",
	}
	if err := ts.Start(); err != nil {
		logger.Fatalf("can't start tsnet server: %v", err)
	}
	defer ts.Close()

	listenAddr := "[::]:" + *listenPort
	logger.Infof("🚀 Starting railtail (ts-hostname=%s, listen-addr=%s, target-addr=%s)", *tsHostname, listenAddr, *targetAddr)

	targetUri, err := url.Parse(*targetAddr)
	if err != nil {
		logger.Fatalf("unable to parse target address: %v", err)
	}

	if targetUri.Scheme == "http" || targetUri.Scheme == "https" {
		// HTTP/s proxy
		logger.Info("running in HTTP/s proxy mode (http(s):// scheme detected in targetAddr)")
		httpClient := ts.HTTPClient()
		httpClient.Timeout = 5 * time.Minute
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpServer := http.Server{
			Addr:              listenAddr,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleHttpConn(logger, httpClient, *targetAddr, w, r)
			}),
		}
		err := httpServer.ListenAndServe()
		if err != nil {
			logger.Fatalf("unable to start http server: %v", err)
		}
	} else {
		// TCP tunnel
		logger.Info("running in TCP tunnel mode (no HTTP scheme detected in targetAddr)")
		listener, err := net.Listen("tcp", listenAddr)
		if err != nil {
			logger.Fatalf("unable to start listener: %v", err)
		}
		for {
			conn, err := listener.Accept()
			if err != nil {
				logger.Errorf("listener accept failed: %v", err)
			}
			go handleTcpConn(logger, conn, ts, *targetAddr)
		}
	}

}
