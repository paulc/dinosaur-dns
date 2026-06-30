package dhcp

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/cache"
	"github.com/paulc/dinosaur-dns/config"
)

// Lease represents a single DHCP lease.
type Lease struct {
	ClientID  string    `json:"client_id"` // hex client-id option or MAC
	MAC       string    `json:"mac"`
	IP        string    `json:"ip"`
	Hostname  string    `json:"hostname"`
	Domain    string    `json:"domain"`
	Expires   time.Time `json:"expires"`
	IsFixed   bool      `json:"is_fixed"`
	Interface string    `json:"interface"`
}

func (l *Lease) Active() bool {
	return l.IsFixed || time.Now().Before(l.Expires)
}

// leaseDB manages leases for one subnet.
type leaseDB struct {
	mu         sync.RWMutex
	byClientID map[string]*Lease
	byIP       map[string]*Lease
	pool       []net.IP // available IPs, in order
	cfg        config.DhcpSubnetConfig
	dnsCache   *cache.DNSCache
	proxyConf  *config.ProxyConfig
}

func newLeaseDB(cfg config.DhcpSubnetConfig, pc *config.ProxyConfig) (*leaseDB, error) {
	db := &leaseDB{
		byClientID: make(map[string]*Lease),
		byIP:       make(map[string]*Lease),
		cfg:        cfg,
		dnsCache:   pc.Cache,
		proxyConf:  pc,
	}

	// Build the dynamic pool: range-start to range-end, excluding fixed IPs.
	start := net.ParseIP(cfg.RangeStart).To4()
	end := net.ParseIP(cfg.RangeEnd).To4()
	if start == nil || end == nil {
		return nil, fmt.Errorf("dhcp %s: invalid range %s-%s", cfg.Interface, cfg.RangeStart, cfg.RangeEnd)
	}
	fixed := make(map[string]bool)
	for _, f := range cfg.Fixed {
		fixed[f.Address] = true
	}
	for ip := cloneIP(start); !ipAfter(ip, end); incrementIP(ip) {
		if !fixed[ip.String()] {
			db.pool = append(db.pool, cloneIP(ip))
		}
	}

	// Load persisted leases.
	if cfg.LeaseFile != "" {
		if err := db.load(); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("dhcp %s: loading lease file: %w", cfg.Interface, err)
		}
	}

	// Register fixed entries (they never expire).
	for _, f := range cfg.Fixed {
		ip := net.ParseIP(f.Address).To4()
		if ip == nil {
			continue
		}
		mac, err := net.ParseMAC(f.MAC)
		if err != nil {
			continue
		}
		cid := macKey(mac)
		l := &Lease{
			ClientID:  cid,
			MAC:       mac.String(),
			IP:        f.Address,
			Hostname:  f.Host,
			Domain:    cfg.DomainName,
			IsFixed:   true,
			Interface: cfg.Interface,
		}
		db.byClientID[cid] = l
		db.byIP[f.Address] = l
		db.addDNSEntry(l)
	}

	return db, nil
}

// allocate returns the IP for a client, creating a new lease if necessary.
// Returns nil if no IP is available.
func (db *leaseDB) allocate(clientID string, mac net.HardwareAddr, hostname string) *Lease {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Existing lease?
	if l, ok := db.byClientID[clientID]; ok {
		if l.IsFixed || time.Now().Before(l.Expires) {
			l.Hostname = hostname
			return l
		}
	}

	// Look for a fixed entry by MAC.
	for _, f := range db.cfg.Fixed {
		fmac, err := net.ParseMAC(f.MAC)
		if err != nil {
			continue
		}
		if fmac.String() == mac.String() {
			cid := macKey(fmac)
			if l, ok := db.byClientID[cid]; ok {
				return l
			}
		}
	}

	// Allocate from pool.
	leaseTime := db.defaultLeaseTime()
	for i, ip := range db.pool {
		if _, used := db.byIP[ip.String()]; used {
			// Might have expired.
			if l := db.byIP[ip.String()]; l.Active() {
				continue
			}
			db.expireLease(db.byIP[ip.String()])
		}
		// Remove from available list.
		db.pool = append(db.pool[:i], db.pool[i+1:]...)
		l := &Lease{
			ClientID:  clientID,
			MAC:       mac.String(),
			IP:        ip.String(),
			Hostname:  hostname,
			Domain:    db.cfg.DomainName,
			Expires:   time.Now().Add(leaseTime),
			Interface: db.cfg.Interface,
		}
		db.byClientID[clientID] = l
		db.byIP[ip.String()] = l
		return l
	}
	return nil
}

// confirm finalises a lease that was offered.
func (db *leaseDB) confirm(clientID string, ip net.IP, hostname string) *Lease {
	db.mu.Lock()
	defer db.mu.Unlock()

	l, ok := db.byClientID[clientID]
	if !ok || l.IP != ip.String() {
		return nil
	}
	if !l.IsFixed {
		l.Expires = time.Now().Add(db.defaultLeaseTime())
		l.Hostname = hostname
	}
	db.addDNSEntry(l)
	db.save()
	return l
}

// renew extends an existing lease for a client that already has an IP.
func (db *leaseDB) renew(clientID string, ip net.IP, hostname string) *Lease {
	db.mu.Lock()
	defer db.mu.Unlock()

	l, ok := db.byClientID[clientID]
	if !ok || l.IP != ip.String() {
		return nil
	}
	if !l.IsFixed {
		l.Expires = time.Now().Add(db.defaultLeaseTime())
	}
	if hostname != "" {
		l.Hostname = hostname
	}
	db.removeDNSEntry(l)
	db.addDNSEntry(l)
	db.save()
	return l
}

// release frees a lease.
func (db *leaseDB) release(clientID string, ip net.IP) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if l, ok := db.byClientID[clientID]; ok && l.IP == ip.String() && !l.IsFixed {
		db.expireLease(l)
		db.save()
	}
}

// markDeclined removes an IP from the pool temporarily (30 min).
func (db *leaseDB) markDeclined(ip net.IP) {
	db.mu.Lock()
	defer db.mu.Unlock()
	dummy := &Lease{
		IP:      ip.String(),
		Expires: time.Now().Add(30 * time.Minute),
	}
	db.byIP[ip.String()] = dummy
}

// deleteByIP removes a lease by IP (admin API).
func (db *leaseDB) deleteByIP(ipStr string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()
	l, ok := db.byIP[ipStr]
	if !ok || l.IsFixed {
		return false
	}
	db.expireLease(l)
	db.save()
	return true
}

// addStatic inserts a lease immediately (admin API).
func (db *leaseDB) addStatic(mac net.HardwareAddr, ip net.IP, hostname string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	cid := macKey(mac)
	if _, ok := db.byIP[ip.String()]; ok {
		return fmt.Errorf("IP %s already allocated", ip)
	}
	l := &Lease{
		ClientID:  cid,
		MAC:       mac.String(),
		IP:        ip.String(),
		Hostname:  hostname,
		Domain:    db.cfg.DomainName,
		Expires:   time.Now().Add(db.defaultLeaseTime()),
		Interface: db.cfg.Interface,
	}
	db.byClientID[cid] = l
	db.byIP[ip.String()] = l
	db.addDNSEntry(l)
	db.save()
	return nil
}

// allLeases returns a snapshot of all active leases.
func (db *leaseDB) allLeases() []Lease {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var out []Lease
	for _, l := range db.byClientID {
		if l.Active() {
			out = append(out, *l)
		}
	}
	return out
}

// reapExpired removes expired leases and cleans up DNS entries.
func (db *leaseDB) reapExpired() {
	db.mu.Lock()
	defer db.mu.Unlock()
	changed := false
	for _, l := range db.byClientID {
		if !l.IsFixed && !time.Now().Before(l.Expires) {
			db.expireLease(l)
			changed = true
		}
	}
	if changed {
		db.save()
	}
}

// expireLease must be called with mu held.
func (db *leaseDB) expireLease(l *Lease) {
	db.removeDNSEntry(l)
	delete(db.byClientID, l.ClientID)
	delete(db.byIP, l.IP)
	// Return IP to pool.
	ip := net.ParseIP(l.IP).To4()
	if ip != nil {
		db.pool = append(db.pool, ip)
	}
}

// DNS helpers — must be called with mu held.

func (db *leaseDB) addDNSEntry(l *Lease) {
	if db.cfg.DomainName == "" || l.Hostname == "" {
		return
	}
	name := strings.ToLower(l.Hostname) + "." + db.cfg.DomainName + "."
	ttl := uint32(3600)
	if !l.IsFixed {
		remaining := time.Until(l.Expires).Seconds()
		if remaining < 60 {
			return
		}
		ttl = uint32(remaining)
	}
	rr, err := dns.NewRR(fmt.Sprintf("%s %d IN A %s", name, ttl, l.IP))
	if err != nil {
		return
	}
	db.dnsCache.AddRR(rr, l.IsFixed)
}

func (db *leaseDB) removeDNSEntry(l *Lease) {
	if db.cfg.DomainName == "" || l.Hostname == "" {
		return
	}
	name := strings.ToLower(l.Hostname) + "." + db.cfg.DomainName + "."
	db.dnsCache.DeleteName(name, "A", false)
}

// Persistence.

func (db *leaseDB) load() error {
	data, err := os.ReadFile(db.cfg.LeaseFile)
	if err != nil {
		return err
	}
	var leases []Lease
	if err := json.Unmarshal(data, &leases); err != nil {
		return err
	}
	for i := range leases {
		l := &leases[i]
		if l.IsFixed || time.Now().Before(l.Expires) {
			db.byClientID[l.ClientID] = l
			db.byIP[l.IP] = l
			// Remove from pool.
			db.pool = removeFromPool(db.pool, net.ParseIP(l.IP).To4())
		}
	}
	return nil
}

func (db *leaseDB) save() {
	if db.cfg.LeaseFile == "" {
		return
	}
	var leases []Lease
	for _, l := range db.byClientID {
		if !l.IsFixed {
			leases = append(leases, *l)
		}
	}
	data, err := json.MarshalIndent(leases, "", "  ")
	if err != nil {
		return
	}
	tmp := db.cfg.LeaseFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	os.Rename(tmp, db.cfg.LeaseFile)
}

// Helpers.

func (db *leaseDB) defaultLeaseTime() time.Duration {
	t := db.cfg.DefaultLeaseTime
	if t <= 0 {
		t = 3600
	}
	return time.Duration(t) * time.Second
}

func (db *leaseDB) maxLeaseTime() time.Duration {
	t := db.cfg.MaxLeaseTime
	if t <= 0 {
		t = 86400
	}
	return time.Duration(t) * time.Second
}

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func ipAfter(a, b net.IP) bool {
	for i := range a {
		if a[i] > b[i] {
			return true
		}
		if a[i] < b[i] {
			return false
		}
	}
	return false
}

func macKey(mac net.HardwareAddr) string {
	return mac.String()
}

func removeFromPool(pool []net.IP, ip net.IP) []net.IP {
	for i, p := range pool {
		if p.Equal(ip) {
			return append(pool[:i], pool[i+1:]...)
		}
	}
	return pool
}
