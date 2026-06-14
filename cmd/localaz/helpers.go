package main

import (
	"log/slog"
	"net"
	"os"
)

// fatal logs what and exits when err is non-nil. It collapses the repeated
// "initialise dependency or die" wiring in main.
func fatal(logger *slog.Logger, what string, err error) {
	if err != nil {
		logger.Error(what, "err", err)
		os.Exit(1)
	}
}

// controlURL builds the externally reachable base URL of a control-plane
// service from the advertise host and the port portion of its listen address.
func controlURL(scheme, host, addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "0"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}
