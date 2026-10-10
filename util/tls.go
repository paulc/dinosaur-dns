package util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

// MakeTLSConfig returns a TLS configuration using the supplied cert/key files.
// The certificate is served via a CertReloader (returned) so it can be
// replaced while running (e.g. after ACME renewal). If both paths are empty a
// fresh self-signed ECDSA-P256 certificate is generated in memory for hosts
// (see GenerateSelfSigned) and the returned CertReloader is nil.
func MakeTLSConfig(certFile, keyFile string, hosts []string) (*tls.Config, *CertReloader, error) {
	// NextProtos enables HTTP/2 negotiation via ALPN; Go's http.Server
	// activates the h2 handler automatically when it sees "h2" here.
	nextProtos := []string{"h2", "http/1.1"}

	if certFile != "" && keyFile != "" {
		reloader, err := NewCertReloader(certFile, keyFile)
		if err != nil {
			return nil, nil, err
		}
		return &tls.Config{GetCertificate: reloader.GetCertificate, NextProtos: nextProtos}, reloader, nil
	}

	cert, err := GenerateSelfSigned(hosts)
	if err != nil {
		return nil, nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: nextProtos}, nil, nil
}

// CertReloader serves a certificate loaded from cert/key files and replaces
// it when Reload finds the files have changed. GetCertificate is just an
// atomic load so there is no per-connection file access.
type CertReloader struct {
	certFile, keyFile string
	cert              atomic.Pointer[tls.Certificate]
}

// NewCertReloader loads the cert/key files
func NewCertReloader(certFile, keyFile string) (*CertReloader, error) {
	r := &CertReloader{certFile: certFile, keyFile: keyFile}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	r.cert.Store(&cert)
	return r, nil
}

// GetCertificate implements tls.Config.GetCertificate
func (r *CertReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return r.cert.Load(), nil
}

// Reload re-reads the cert/key files and swaps in the certificate if it has
// changed. On error (e.g. a cert/key mismatch part way through renewal) the
// current certificate is kept.
func (r *CertReloader) Reload() (changed bool, err error) {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return false, err
	}
	if slices.EqualFunc(cert.Certificate, r.cert.Load().Certificate, slices.Equal) {
		return false, nil
	}
	r.cert.Store(&cert)
	return true, nil
}

// NotAfter returns the expiry time of the current certificate
func (r *CertReloader) NotAfter() time.Time {
	return r.cert.Load().Leaf.NotAfter
}

// WriteCertPair writes cert as PEM cert/key files
func WriteCertPair(cert tls.Certificate, certFile, keyFile string) error {
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return err
	}
	var certPEM []byte
	for _, der := range cert.Certificate {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		return err
	}
	return os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600)
}

// GenerateSelfSigned returns an in-memory self-signed ECDSA-P256 certificate
// with hosts (IP addresses or hostnames) as subject alternative names.
// Unspecified addresses (0.0.0.0, ::) are skipped and IPv6 zones removed.
func GenerateSelfSigned(hosts []string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "dinosaur-dns"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		h, _, _ = strings.Cut(h, "%")
		if ip := net.ParseIP(h); ip != nil {
			if !ip.IsUnspecified() && !slices.ContainsFunc(tmpl.IPAddresses, ip.Equal) {
				tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
			}
		} else if h != "" && !slices.Contains(tmpl.DNSNames, h) {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	)
}
