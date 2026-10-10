package util

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSelfSigned(t *testing.T) {
	cert, err := GenerateSelfSigned()
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("empty certificate")
	}
}

func TestMakeTLSConfig(t *testing.T) {
	cfg, err := MakeTLSConfig("", "")
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
	cert, err := GenerateSelfSigned()
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

	cfg, err := MakeTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) != 1 || string(cfg.Certificates[0].Certificate[0]) != string(cert.Certificate[0]) {
		t.Fatal("loaded certificate does not match")
	}

	if _, err := MakeTLSConfig(certFile, filepath.Join(dir, "missing.pem")); err == nil {
		t.Fatal("expected error for missing key")
	}
}
