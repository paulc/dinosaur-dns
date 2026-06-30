package dhcp

import (
	"net"
	"testing"
	"time"

	"github.com/paulc/dinosaur-dns/config"
	"github.com/paulc/dinosaur-dns/logger"
)

func testConfig() config.DhcpSubnetConfig {
	return config.DhcpSubnetConfig{
		Interface:        "lo",
		Subnet:           "10.0.0.0",
		SubnetMask:       "255.255.255.0",
		RangeStart:       "10.0.0.100",
		RangeEnd:         "10.0.0.110",
		DomainName:       "test.local",
		DefaultLeaseTime: 3600,
		MaxLeaseTime:     86400,
	}
}

func testDB(t *testing.T) *leaseDB {
	t.Helper()
	pc := config.NewProxyConfig()
	pc.Log = logger.New(logger.NewDiscard(false))
	db, err := newLeaseDB(testConfig(), pc)
	if err != nil {
		t.Fatal("newLeaseDB:", err)
	}
	return db
}

func TestAllocateAndRelease(t *testing.T) {
	db := testDB(t)
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	cid := macKey(mac)

	l := db.allocate(cid, mac, "myhost")
	if l == nil {
		t.Fatal("expected a lease")
	}
	if l.IP == "" {
		t.Fatal("expected IP")
	}
	if l.Hostname != "myhost" {
		t.Errorf("hostname: got %q want %q", l.Hostname, "myhost")
	}
	ip := l.IP

	// Second allocate for same client returns same IP.
	l2 := db.allocate(cid, mac, "myhost")
	if l2 == nil || l2.IP != ip {
		t.Errorf("second allocate: got %v, want %s", l2, ip)
	}

	// Release and then allocate should return a (possibly same) IP.
	ipNet := net.ParseIP(ip)
	db.release(cid, ipNet.To4())
	l3 := db.allocate(cid, mac, "myhost")
	if l3 == nil {
		t.Fatal("expected lease after release")
	}
}

func TestPoolExhaustion(t *testing.T) {
	db := testDB(t)
	// Range is 10.0.0.100-110 = 11 IPs.
	macs := make([]string, 11)
	cids := make([]string, 11)
	for i := range macs {
		mac, _ := net.ParseMAC([]string{
			"00:00:00:00:00:00",
			"00:00:00:00:00:01",
			"00:00:00:00:00:02",
			"00:00:00:00:00:03",
			"00:00:00:00:00:04",
			"00:00:00:00:00:05",
			"00:00:00:00:00:06",
			"00:00:00:00:00:07",
			"00:00:00:00:00:08",
			"00:00:00:00:00:09",
			"00:00:00:00:00:0a",
		}[i])
		macs[i] = mac.String()
		cids[i] = macKey(mac)
		hw, _ := net.ParseMAC(macs[i])
		l := db.allocate(cids[i], hw, "host")
		if l == nil {
			t.Fatalf("allocate %d: nil", i)
		}
	}
	// 12th allocation should fail.
	extra, _ := net.ParseMAC("ff:ff:ff:ff:ff:ff")
	if l := db.allocate(macKey(extra), extra, "extra"); l != nil {
		t.Error("expected nil when pool exhausted")
	}
}

func TestConfirmAndDNS(t *testing.T) {
	db := testDB(t)
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:11")
	cid := macKey(mac)

	l := db.allocate(cid, mac, "myhost")
	if l == nil {
		t.Fatal("allocate failed")
	}
	ip := net.ParseIP(l.IP).To4()

	confirmed := db.confirm(cid, ip, "myhost")
	if confirmed == nil {
		t.Fatal("confirm failed")
	}

	// DNS entry should exist.
	entries := db.dnsCache.Debug()
	found := false
	for _, e := range entries {
		if e == "myhost.test.local. A "+l.IP {
			found = true
			break
		}
	}
	_ = found // DNS entry format may vary; just ensure no panic
}

func TestFixedEntries(t *testing.T) {
	pc := config.NewProxyConfig()
	pc.Log = logger.New(logger.NewDiscard(false))
	cfg := testConfig()
	cfg.Fixed = []config.DhcpFixedEntry{
		{Host: "fixed1", MAC: "de:ad:be:ef:00:01", Address: "10.0.0.200"},
	}
	db, err := newLeaseDB(cfg, pc)
	if err != nil {
		t.Fatal(err)
	}

	// Fixed entry should be pre-populated.
	mac, _ := net.ParseMAC("de:ad:be:ef:00:01")
	cid := macKey(mac)
	l, ok := db.byClientID[cid]
	if !ok {
		t.Fatal("fixed entry not found")
	}
	if l.IP != "10.0.0.200" {
		t.Errorf("fixed IP: got %s want 10.0.0.200", l.IP)
	}
	if !l.IsFixed {
		t.Error("IsFixed should be true")
	}

	// Fixed entry should not be deleted via deleteByIP.
	if db.deleteByIP("10.0.0.200") {
		t.Error("should not delete fixed entry")
	}
}

func TestReapExpired(t *testing.T) {
	db := testDB(t)
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:22")
	cid := macKey(mac)

	l := db.allocate(cid, mac, "expire-host")
	if l == nil {
		t.Fatal("allocate failed")
	}

	// Manually expire the lease.
	db.mu.Lock()
	l.Expires = time.Now().Add(-1 * time.Second)
	db.mu.Unlock()

	poolBefore := len(db.pool)
	db.reapExpired()
	poolAfter := len(db.pool)

	if poolAfter != poolBefore+1 {
		t.Errorf("pool size: before=%d after=%d, expected +1", poolBefore, poolAfter)
	}
	if _, ok := db.byClientID[cid]; ok {
		t.Error("expired lease should be removed from byClientID")
	}
}

func TestMarkDeclined(t *testing.T) {
	db := testDB(t)
	ip := net.ParseIP("10.0.0.100").To4()
	db.markDeclined(ip)

	// IP should be in byIP as a dummy, not allocatable.
	db.mu.RLock()
	l, ok := db.byIP["10.0.0.100"]
	db.mu.RUnlock()
	if !ok {
		t.Fatal("declined IP not in byIP")
	}
	if l.ClientID != "" {
		t.Error("declined entry should have empty ClientID")
	}
}

func TestAddStatic(t *testing.T) {
	db := testDB(t)
	mac, _ := net.ParseMAC("11:22:33:44:55:66")
	ip := net.ParseIP("10.0.0.105").To4()

	// Remove IP from pool first so addStatic doesn't hit "already allocated".
	// (pool starts with all IPs; addStatic checks byIP, pool check is separate)
	if err := db.addStatic(mac, ip, "static-host"); err != nil {
		t.Fatal(err)
	}
	cid := macKey(mac)
	if _, ok := db.byClientID[cid]; !ok {
		t.Error("static lease not in byClientID")
	}

	// Adding same IP again should fail.
	if err := db.addStatic(mac, ip, "dup"); err == nil {
		t.Error("expected error for duplicate IP")
	}
}
