package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func createQuery(qname string, qtype string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(qname, dns.StringToType[qtype])
	return msg
}

func createCacheItem(qname string, qtype string, answer string) (msg *dns.Msg, err error) {

	rr, err := dns.NewRR(answer)
	if err != nil {
		return nil, fmt.Errorf("Error creating RR: %s", err)
	}

	msg = new(dns.Msg)
	msg.SetQuestion(qname, dns.StringToType[qtype])
	msg.Response = true
	msg.Authoritative = true
	msg.RecursionAvailable = true
	msg.Rcode = dns.RcodeSuccess
	msg.Answer = append(msg.Answer, rr)

	return msg, nil
}

func TestAdd(t *testing.T) {

	// Use mock time.Now
	now := time.Now()
	timeNow = func() time.Time {
		return now
	}

	defer func() {
		timeNow = time.Now
	}()

	cache := NewDNSCache()

	// Insert entries
	for i := 0; i < 100; i++ {
		msg, err := createCacheItem(fmt.Sprintf("%04d.test.com", i), "A", "0001.test.com. 60 IN A 1.2.3.4")
		if err != nil {
			t.Error(err)
		}
		cache.Add(msg)
	}

	if len(cache.Cache) != 100 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}
	t.Logf("Cache: %d items", len(cache.Cache))

	// Get from cache
	for i := 0; i < 100; i++ {
		q := createQuery(fmt.Sprintf("%04d.test.com", i), "A")
		_, found := cache.Get(q)
		if !found {
			t.Errorf("%04d.test.com not found", i)
		}
	}

	// Jump forward time
	now = now.Add(time.Second * 100)
	for i := 0; i < 100; i++ {
		q := createQuery(fmt.Sprintf("%04d.test.com", i), "A")
		_, found := cache.Get(q)
		if found {
			t.Errorf("%04d.test.com found (should be expired)", i)
		}
	}

	if len(cache.Cache) != 0 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}
	t.Logf("Cache: %d items", len(cache.Cache))
}

func TestExpire(t *testing.T) {

	// Use mock time.Now
	now := time.Now()
	timeNow = func() time.Time {
		return now
	}

	defer func() {
		timeNow = time.Now
	}()

	cache := NewDNSCache()

	// Insert entries
	for i := 0; i < 100; i++ {
		msg, err := createCacheItem(fmt.Sprintf("%04d.test.com", i), "A", "0001.test.com. 60 IN A 1.2.3.4")
		if err != nil {
			t.Error(err)
		}
		cache.Add(msg)
	}

	if len(cache.Cache) != 100 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}
	t.Logf("Cache: %d items", len(cache.Cache))

	// Shouldnt flush any entries
	total, expired := cache.Flush()
	if len(cache.Cache) != 100 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}
	t.Logf("Cache: %d/%d items", total, expired)

	// Jump forward time
	now = now.Add(time.Second * 100)
	// Should flush all entries
	total, expired = cache.Flush()
	if len(cache.Cache) != 0 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}
	t.Logf("Cache: %d/%d items", total, expired)
}

func TestConcurrent(t *testing.T) {

	cache := NewDNSCache()
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		for i := 0; i < 1000000; i++ {
			msg, _ := createCacheItem(fmt.Sprintf("%08d.test.com", i), "A", "0001.test.com. 5 IN A 1.2.3.4")
			cache.Add(msg)
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 1000000; i++ {
			q := createQuery(fmt.Sprintf("%08d.test.com", i), "A")
			cache.Get(q)
		}
		wg.Done()
	}()

	go func() {
		for {
			total, expired := cache.Flush()
			t.Logf("Cache: %d/%d (total/expired)", total, expired)
			time.Sleep(time.Millisecond * 500)
		}
	}()

	wg.Wait()
}

func TestAddPermanent(t *testing.T) {

	cache := NewDNSCache()
	err := cache.AddPermanent("abc.def.com 60 A 1.2.3.4")
	if err != nil {
		t.Error(err)
	}

	q := createQuery("abc.def.com.", "A")

	_, found := cache.Get(q)

	if found == false {
		t.Errorf("AddPermanent :: not found")
	}
}
