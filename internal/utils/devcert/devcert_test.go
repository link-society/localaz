package devcert

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
)

// parseCert decodes the returned PEM and parses the embedded certificate.
func parseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatalf("decode cert PEM: no block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func hasDNS(cert *x509.Certificate, name string) bool {
	for _, n := range cert.DNSNames {
		if n == name {
			return true
		}
	}
	return false
}

func hasIP(cert *x509.Certificate, ip net.IP) bool {
	for _, got := range cert.IPAddresses {
		if got.Equal(ip) {
			return true
		}
	}
	return false
}

func TestGenerateIncludesProvidedHosts(t *testing.T) {
	certPEM, _, err := Generate("example.test", "10.1.2.3")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	cert := parseCert(t, certPEM)

	// Loopback defaults must still be present.
	if !hasDNS(cert, "localhost") {
		t.Errorf("DNSNames %v missing default localhost", cert.DNSNames)
	}
	if !hasIP(cert, net.IPv4(127, 0, 0, 1)) {
		t.Errorf("IPAddresses %v missing 127.0.0.1", cert.IPAddresses)
	}
	if !hasIP(cert, net.IPv6loopback) {
		t.Errorf("IPAddresses %v missing ::1", cert.IPAddresses)
	}

	// Provided hosts must be added: hostname into DNSNames, IP into IPAddresses.
	if !hasDNS(cert, "example.test") {
		t.Errorf("DNSNames %v missing provided host example.test", cert.DNSNames)
	}
	if !hasIP(cert, net.ParseIP("10.1.2.3")) {
		t.Errorf("IPAddresses %v missing provided host 10.1.2.3", cert.IPAddresses)
	}
}

func TestGenerateNoArgsKeepsLoopbackDefaults(t *testing.T) {
	certPEM, _, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	cert := parseCert(t, certPEM)

	if !hasDNS(cert, "localhost") {
		t.Errorf("DNSNames %v missing localhost", cert.DNSNames)
	}
	if !hasIP(cert, net.IPv4(127, 0, 0, 1)) {
		t.Errorf("IPAddresses %v missing 127.0.0.1", cert.IPAddresses)
	}
	if !hasIP(cert, net.IPv6loopback) {
		t.Errorf("IPAddresses %v missing ::1", cert.IPAddresses)
	}
}
