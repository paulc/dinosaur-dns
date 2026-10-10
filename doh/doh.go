package doh

import (
	"encoding/base64"
	"io"
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

// StartDoH starts one TLS listener per address in config.DohBind (using the
// shared ssl-cert/ssl-key TLS config).
// Timeouts prevent hung idle connections while keeping HTTP keep-alive and
// HTTP/2 multiplexing functional for typical DoH clients.
func StartDoH(proxyConfig *config.ProxyConfig) {
	log := proxyConfig.Log

	tlsCfg, err := proxyConfig.GetTLSConfig()
	if err != nil {
		log.Fatalf("DoH TLS: %s", err)
	}

	if proxyConfig.SslCert == "" {
		log.Printf("DoH: using auto-generated self-signed certificate")
	}

	handler := MakeDoHHandler(proxyConfig)

	for _, addr := range proxyConfig.DohBind {
		addr := addr
		srv := &http.Server{
			Addr:      addr,
			Handler:   handler,
			TLSConfig: tlsCfg,
			// ReadHeaderTimeout guards against Slowloris-style header stalls.
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			// IdleTimeout allows keep-alive connections to be reused while
			// releasing clients that have gone away without closing cleanly.
			IdleTimeout: 120 * time.Second,
		}
		// Bind synchronously so a failure (e.g. missing interface) is logged
		// and skipped rather than being fatal
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("Error: DoH listener %s: %s", addr, err)
			continue
		}
		go func() {
			log.Printf("Starting DoH listener: %s%s", addr, proxyConfig.DohPath)
			// Empty cert/key paths are correct here: TLSConfig.Certificates is
			// already populated, so ServeTLS skips the LoadX509KeyPair call.
			if err := srv.ServeTLS(ln, "", ""); err != nil {
				log.Printf("Error: DoH listener %s: %s", addr, err)
			}
		}()
	}
}
