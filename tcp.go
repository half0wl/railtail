package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/half0wl/railtail/internal/util"
	"github.com/northbright/iocopy"
	"golang.org/x/sync/errgroup"
	"tailscale.com/tsnet"
)

func fwdTCP(lstConn net.Conn, ts *tsnet.Server, targetAddr string) error {
	defer lstConn.Close()

	if tcpConn, ok := lstConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsConn, err := ts.Dial(ctx, "tcp", targetAddr)
	if err != nil {
		return fmt.Errorf("failed to dial tailscale node: %w", err)
	}

	defer tsConn.Close()

	if tcpConn, ok := tsConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer cancel()

		defer func() {
			if tcpConn, ok := tsConn.(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			}
		}()

		if _, err := iocopy.Copy(ctx, tsConn, lstConn); err != nil && !util.IsExpectedCopyError(err) {
			return fmt.Errorf("failed to copy data to target: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		defer cancel()

		defer func() {
			if tcpConn, ok := lstConn.(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			}
		}()

		if _, err := iocopy.Copy(ctx, lstConn, tsConn); err != nil && !util.IsExpectedCopyError(err) {
			return fmt.Errorf("failed to copy data from source: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	return nil
}
