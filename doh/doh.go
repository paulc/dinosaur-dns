package doh

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/proxy"
)

// dohAddr implements net.Addr for use in the fake dns.ResponseWriter.
type dohAddr struct {
	network string
	addr    string
}

func (a dohAddr) Network() string { return a.network }
func (a dohAddr) String() string  { return a.addr }

// dohResponseWriter adapts the DoH HTTP context into a dns.ResponseWriter so
// the existing proxy handler can be used without modification.
type dohResponseWriter struct {
	local  net.Addr
	remote net.Addr
	msg    *dns.Msg
}

func (w *dohResponseWriter) LocalAddr() net.Addr         { return w.local }
func (w *dohResponseWriter) RemoteAddr() net.Addr        { return w.remote }
func (w *dohResponseWriter) WriteMsg(m *dns.Msg) error   { w.msg = m; return nil }
func (w *dohResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *dohResponseWriter) Close() error                { return nil }
func (w *dohResponseWriter) TsigStatus() error           { return nil }
func (w *dohResponseWriter) TsigTimersOnly(bool)         {}
func (w *dohResponseWriter) Hijack()                     {}

// MakeDoHHandler returns an http.Handler implementing RFC 8484 DoH.
// Both GET (?dns=<base64url>) and POST (application/dns-message body) are
// supported. All proxy logic (blocklist, cache, ACL, DNS64) applies exactly
// as it does for UDP/TCP clients.
func MakeDoHHandler(proxyConfig *config.ProxyConfig) http.Handler {
	dnsHandler := proxy.MakeHandler(proxyConfig)
	path := proxyConfig.DohPath

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}

		var msgBytes []byte
		var err error

		switch r.Method {
		case http.MethodGet:
			b64 := r.URL.Query().Get("dns")
			if b64 == "" {
				http.Error(w, "missing dns parameter", http.StatusBadRequest)
				return
			}
			msgBytes, err = base64.RawURLEncoding.DecodeString(b64)
			if err != nil {
				http.Error(w, "invalid dns parameter", http.StatusBadRequest)
				return
			}

		case http.MethodPost:
			if r.Header.Get("Content-Type") != "application/dns-message" {
				http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
				return
			}
			msgBytes, err = io.ReadAll(io.LimitReader(r.Body, 65536))
			if err != nil {
				http.Error(w, "error reading body", http.StatusBadRequest)
				return
			}

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		q := new(dns.Msg)
		if err := q.Unpack(msgBytes); err != nil {
			http.Error(w, "invalid dns message", http.StatusBadRequest)
			return
		}

		dohW := &dohResponseWriter{
			local:  dohAddr{"tcp", r.Host},
			remote: dohAddr{"tcp", r.RemoteAddr},
		}
		dnsHandler(dohW, q)

		if dohW.msg == nil {
			// Proxy dropped the request (e.g. ACL rejection).
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		respBytes, err := dohW.msg.Pack()
		if err != nil {
			http.Error(w, "error packing response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/dns-message")
		w.WriteHeader(http.StatusOK)
		w.Write(respBytes) //nolint:errcheck
	})
}

// MakeTLSConfig returns a TLS configuration using the supplied cert/key files.
// If both paths are empty a fresh self-signed ECDSA-P256 certificate is
// generated in memory.
func MakeTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	var cert tls.Certificate
	var err error

	if certFile != "" && keyFile != "" {
		cert, err = tls.LoadX509KeyPair(certFile, keyFile)
	} else {
		cert, err = generateSelfSigned()
	}
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		// NextProtos enables HTTP/2 negotiation via ALPN; Go's http.Server
		// activates the h2 handler automatically when it sees "h2" here.
		NextProtos: []string{"h2", "http/1.1"},
	}, nil
}

func generateSelfSigned() (tls.Certificate, error) {
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

// StartDoH starts one TLS listener per address in config.DohBind.
// Timeouts prevent hung idle connections while keeping HTTP keep-alive and
// HTTP/2 multiplexing functional for typical DoH clients.
func StartDoH(proxyConfig *config.ProxyConfig) {
	log := proxyConfig.Log

	tlsCfg, err := MakeTLSConfig(proxyConfig.DohCert, proxyConfig.DohKey)
	if err != nil {
		log.Fatalf("DoH TLS: %s", err)
	}

	if proxyConfig.DohCert == "" {
		log.Printf("DoH: using auto-generated self-signed certificate")
	}

	handler := MakeDoHHandler(proxyConfig)

	for _, addr := range proxyConfig.DohBind {
		addr := addr
		srv := &http.Server{
			Addr:    addr,
			Handler: handler,
			TLSConfig: tlsCfg,
			// ReadHeaderTimeout guards against Slowloris-style header stalls.
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			// IdleTimeout allows keep-alive connections to be reused while
			// releasing clients that have gone away without closing cleanly.
			IdleTimeout: 120 * time.Second,
		}
		go func() {
			log.Printf("Starting DoH listener: %s%s", addr, proxyConfig.DohPath)
			// Empty cert/key paths are correct here: TLSConfig.Certificates is
			// already populated, so ServeTLS skips the LoadX509KeyPair call.
			if err := srv.ListenAndServeTLS("", ""); err != nil {
				log.Fatalf("DoH %s: %s", addr, err)
			}
		}()
	}
}
