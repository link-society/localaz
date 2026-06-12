// Package devcert generates the self-signed TLS material that localaz uses for
// its control-plane (Entra ID / ARM) and bearer data-plane listeners.
//
// The Azure CLI and the Azure SDKs refuse to send bearer tokens over plain
// HTTP, so those endpoints must be served over TLS even locally. The cert is
// intentionally throwaway: it is only ever meant to be trusted explicitly (for
// example via REQUESTS_CA_BUNDLE) by clients talking to a local emulator.
package devcert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// Generate produces a self-signed certificate valid for localhost and the
// loopback addresses, plus any additional hosts supplied. Each host is parsed:
// IP literals are added to the SAN IP addresses, everything else to the DNS
// names. Empty and duplicate hosts are skipped. It returns the PEM-encoded
// certificate and private key.
func Generate(hosts ...string) (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial: %w", err)
	}

	dnsNames := []string{"localhost"}
	ipAddresses := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	for _, host := range hosts {
		if host == "" {
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			if !containsIP(ipAddresses, ip) {
				ipAddresses = append(ipAddresses, ip)
			}
			continue
		}
		if !containsString(dnsNames, host) {
			dnsNames = append(dnsNames, host)
		}
	}

	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "localaz", Organization: []string{"localaz"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal key: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

func containsString(s []string, v string) bool {
	for _, got := range s {
		if got == v {
			return true
		}
	}
	return false
}

func containsIP(s []net.IP, v net.IP) bool {
	for _, got := range s {
		if got.Equal(v) {
			return true
		}
	}
	return false
}
