package main

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"localaz.dev/internal/utils/devcert"
)

// loadTLS resolves the certificate that every HTTP service is served with. An
// explicit cert/key pair wins; otherwise a throwaway self-signed certificate
// (with advertiseHost added to its SANs) is generated and its PEM written under
// <data>/tls so clients can trust it (for example via REQUESTS_CA_BUNDLE).
func loadTLS(logger *slog.Logger, dataDir, certFile, keyFile string, advertiseHost string) (*tls.Certificate, error) {
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load tls key pair: %w", err)
		}
		return &cert, nil
	}

	certPEM, keyPEM, err := devcert.Generate(advertiseHost)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse generated certificate: %w", err)
	}

	tlsDir := filepath.Join(dataDir, "tls")
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tls dir: %w", err)
	}
	certPath := filepath.Join(tlsDir, "localaz.crt")
	keyPath := filepath.Join(tlsDir, "localaz.key")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write certificate: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	logger.Info("generated self-signed certificate", "cert", certPath, "key", keyPath)
	return &cert, nil
}
