package util

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSelfSigned(t *testing.T) {
	cert, err := GenerateSelfSigned([]string{"127.0.0.1", "::1", "fe80::1%eth0", "0.0.0.0", "::", "dns.example.com", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("empty certificate")
	}
	x, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(x.IPAddresses) != 3 || len(x.DNSNames) != 1 {
		t.Errorf("SANs: %v %v", x.IPAddresses, x.DNSNames)
	}
	for _, h := range []string{"127.0.0.1", "::1", "fe80::1", "dns.example.com"} {
		if err := x.VerifyHostname(h); err != nil {
			t.Error(err)
		}
	}
	if err := x.VerifyHostname("10.0.0.1"); err == nil {
		t.Error("Expected error for host not in certificate")
	}
}

func TestMakeTLSConfig(t *testing.T) {
	cfg, err := MakeTLSConfig("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) == 0 {
		t.Fatal("no certificates in TLS config")
	}
	if len(cfg.NextProtos) == 0 {
		t.Fatal("no NextProtos in TLS config")
	}
}

func TestMakeTLSConfigFiles(t *testing.T) {
	// Write a generated cert/key pair to disk and load it back
	cert, err := GenerateSelfSigned(nil)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0600)
	os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600)

	cfg, err := MakeTLSConfig(certFile, keyFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) != 1 || string(cfg.Certificates[0].Certificate[0]) != string(cert.Certificate[0]) {
		t.Fatal("loaded certificate does not match")
	}

	if _, err := MakeTLSConfig(certFile, filepath.Join(dir, "missing.pem"), nil); err == nil {
		t.Fatal("expected error for missing key")
	}
}
