package main

import (
	"context"
	"fmt"
	"io"
	"net"

	"golang.org/x/sync/errgroup"
	"tailscale.com/tsnet"
)

func fwdTCP(lstConn net.Conn, ts *tsnet.Server, targetAddr string) error {
	defer lstConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsConn, err := ts.Dial(ctx, "tcp", targetAddr)
	if err != nil {
		return fmt.Errorf("failed to dial tailscale node: %w", err)
	}

	defer tsConn.Close()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		_, err := io.Copy(tsConn, lstConn)
		return fmt.Errorf("failed to copy data to tailscale node: %w", err)
	})

	g.Go(func() error {
		_, err := io.Copy(lstConn, tsConn)
		return fmt.Errorf("failed to copy data from tailscale node: %w", err)
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	return nil
}
