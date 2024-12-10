package main

import (
	"cmp"
	"context"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"go.uber.org/zap"
	"tailscale.com/tsnet"
)

var (
	tsHostname = flag.String("ts-hostname", "", "hostname to use for tailscale (or set env: TS_HOSTNAME)")
	listenPort = flag.String("listen-port", "", "port to listen on (or set env: LISTEN_PORT)")
	targetAddr = flag.String("target-addr", "", "address:port of a tailscale node to send traffic to (or set env: TARGET_ADDR)")
	tsAuthKey  = cmp.Or(os.Getenv("TS_AUTH_KEY"), "")
)

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
	listener, err := net.Listen("tcp", listenAddr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Errorf("listener accept failed: %v", err)
		}
		go fwd(logger, conn, ts, *targetAddr)
	}
}
