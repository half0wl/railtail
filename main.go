package main

import (
	"cmp"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/half0wl/railtail/internal/config"
	"github.com/half0wl/railtail/internal/logger"

	"tailscale.com/tsnet"
)

func main() {
	cfg, errs := config.LoadConfig()
	if len(errs) > 0 {
		logger.StderrWithSource.Error("configuration error(s) found", logger.ErrorsAttr(errs...))
		os.Exit(1)
	}

	ts := &tsnet.Server{
		Hostname:     cfg.TSHostname,
		AuthKey:      cfg.TSAuthKey,
		RunWebClient: false,
		Ephemeral:    false,
		ControlURL:   cfg.TSLoginServer,
		UserLogf: func(format string, v ...any) {
			logger.Stdout.Info(fmt.Sprintf(format, v...))
		},
		Dir: filepath.Join(cfg.TSStateDirPath, "railtail"),
	}
	if err := ts.Start(); err != nil {
		logger.StderrWithSource.Error("failed to start tailscale network server", logger.ErrAttr(err))
		os.Exit(1)
	}

	defer ts.Close()

	listenAddr := "[::]:" + cfg.ListenPort

	logger.Stdout.Info("🚀 Starting railtail",
		slog.String("ts-hostname", cfg.TSHostname),
		slog.String("listen-addr", listenAddr),
		slog.String("target-addr", cfg.TargetAddr),
		slog.String("ts-login-server", cmp.Or(cfg.TSLoginServer, "using_default")),
		slog.String("ts-state-dir", filepath.Join(cfg.TSStateDirPath, "railtail")),
	)

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.StderrWithSource.Error("failed to start local listener", logger.ErrAttr(err))
		os.Exit(1)
	}

	if cfg.ForwardTrafficType == config.ForwardTrafficTypeHTTP || cfg.ForwardTrafficType == config.ForwardTrafficTypeHTTPS {
		logger.Stdout.Info("running in HTTP/s proxy mode (http(s):// scheme detected in targetAddr)",
			slog.String("listen-addr", listenAddr),
			slog.String("target-addr", cfg.TargetAddr),
		)

		httpClient := ts.HTTPClient()
		httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		server := http.Server{
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				forwardingInfo := []any{
					slog.String("remote-addr", r.RemoteAddr),
					slog.String("target", cfg.TargetAddr),
				}

				logger.Stdout.Info("forwarding", forwardingInfo...)

				if err := fwdHttp(httpClient, cfg.TargetAddr, w, r); err != nil {
					logger.StderrWithSource.Error("failed to forward http request", append([]any{logger.ErrAttr(err)}, forwardingInfo...)...)
				}
			}),
		}

		if err := server.Serve(listener); err != nil {
			logger.StderrWithSource.Error("failed to start http server", logger.ErrAttr(err))
			os.Exit(1)
		}
	}

	logger.Stdout.Info("running in TCP tunnel mode (no HTTP scheme detected in targetAddr)",
		slog.String("listen-addr", listenAddr),
		slog.String("target-addr", cfg.TargetAddr),
	)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.StderrWithSource.Error("failed to accept connection", logger.ErrAttr(err))
			continue
		}

		forwardingInfo := []any{
			slog.String("local-addr", conn.LocalAddr().String()),
			slog.String("remote-addr", conn.RemoteAddr().String()),
			slog.String("target", cfg.TargetAddr),
		}

		logger.Stdout.Info("forwarding tcp connection", forwardingInfo...)

		go func() {
			if err := fwdTCP(conn, ts, cfg.TargetAddr); err != nil {
				logger.StderrWithSource.Error("forwarding failed", append([]any{logger.ErrAttr(err)}, forwardingInfo...)...)
			}
		}()
	}
}
