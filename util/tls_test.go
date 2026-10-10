package util

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	cfg, reloader, err := MakeTLSConfig("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) == 0 || reloader != nil {
		t.Fatal("expected static self-signed certificate")
	}
	if len(cfg.NextProtos) == 0 {
		t.Fatal("no NextProtos in TLS config")
	}
}

func writeTestCert(t *testing.T, certFile, keyFile string) tls.Certificate {
	t.Helper()
	cert, err := GenerateSelfSigned([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCertPair(cert, certFile, keyFile); err != nil {
		t.Fatal(err)
	}
	return cert
}

func servedCert(t *testing.T, cfg *tls.Config) []byte {
	t.Helper()
	cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatal(err)
	}
	return cert.Certificate[0]
}

func TestMakeTLSConfigFiles(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	cert1 := writeTestCert(t, certFile, keyFile)

	cfg, reloader, err := MakeTLSConfig(certFile, keyFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reloader == nil || len(cfg.Certificates) != 0 {
		t.Fatal("expected certificate served via CertReloader")
	}
	if !bytes.Equal(servedCert(t, cfg), cert1.Certificate[0]) {
		t.Fatal("loaded certificate does not match")
	}
	if !reloader.NotAfter().After(time.Now()) {
		t.Errorf("NotAfter: %s", reloader.NotAfter())
	}

	// Unchanged files
	if changed, err := reloader.Reload(); changed || err != nil {
		t.Errorf("Reload unchanged: %v %v", changed, err)
	}

	// Renewed certificate is swapped in
	cert2 := writeTestCert(t, certFile, keyFile)
	if changed, err := reloader.Reload(); !changed || err != nil {
		t.Errorf("Reload renewed: %v %v", changed, err)
	}
	if !bytes.Equal(servedCert(t, cfg), cert2.Certificate[0]) {
		t.Fatal("renewed certificate not served")
	}

	// Cert/key mismatch (e.g. part way through renewal) keeps the current cert
	other := filepath.Join(dir, "other")
	writeTestCert(t, other+".pem", other+".key")
	os.Rename(other+".pem", certFile)
	if _, err := reloader.Reload(); err == nil {
		t.Error("expected error for cert/key mismatch")
	}
	if !bytes.Equal(servedCert(t, cfg), cert2.Certificate[0]) {
		t.Fatal("current certificate not kept after failed reload")
	}

	if _, _, err := MakeTLSConfig(certFile, filepath.Join(dir, "missing.pem"), nil); err == nil {
		t.Fatal("expected error for missing key")
	}
}
