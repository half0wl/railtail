package main

import (
	"context"
	"flag"
	"go.uber.org/zap"
	"io"
	"log"
	"net"
	"os"
	"tailscale.com/tsnet"
)

var (
	tsHostname = flag.String("ts-hostname", "", "hostname to use for tailscale (or set env: TS_HOSTNAME)")
	listenAddr = flag.String("listen-addr", "", "address:port to listen on (or set env: LISTEN_ADDR)")
	targetAddr = flag.String("target-addr", "", "address:port of a tailscale peer to send traffic to (or set env: TARGET_ADDR)")
	tsAuthKey  = getEnv("TS_AUTH_KEY", "")
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func fwd(logger *zap.SugaredLogger, lstConn net.Conn, ts *tsnet.Server, target string) {
	defer lstConn.Close()

	logger.Infof("fwd: %s -> %s -> %s", lstConn.LocalAddr(), lstConn.RemoteAddr(), target)

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

	if *listenAddr == "" {
		*listenAddr = os.Getenv("LISTEN_ADDR")
	}
	if *listenAddr == "" {
		logger.Fatal("listen-addr is required (set LISTEN_ADDR in env or use -listen-addr)")
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

	logger.Infof("🚀 Starting railtail (ts-hostname=%s, listen=%s, target=%s)", *tsHostname, *listenAddr, *targetAddr)
	listener, err := net.Listen("tcp", *listenAddr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Errorf("listener accept failed: %v", err)
		}
		go fwd(logger, conn, ts, *targetAddr)
	}
}
