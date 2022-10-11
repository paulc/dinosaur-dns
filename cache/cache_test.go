package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur-dns/util"
)

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

	cache := New()

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

	// Get from cache
	for i := 0; i < 100; i++ {
		q := util.CreateQuery(fmt.Sprintf("%04d.test.com", i), "A")
		_, found := cache.Get(q)
		if !found {
			t.Errorf("%04d.test.com not found", i)
		}
	}

	// Jump forward time
	now = now.Add(time.Second * 100)
	for i := 0; i < 100; i++ {
		q := util.CreateQuery(fmt.Sprintf("%04d.test.com", i), "A")
		_, found := cache.Get(q)
		if found {
			t.Errorf("%04d.test.com found (should be expired)", i)
		}
	}

	if len(cache.Cache) != 0 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}
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

	cache := New()

	// Insert entries
	for i := 0; i < 100; i++ {
		msg, err := createCacheItem(fmt.Sprintf("%04d.test.com", i), "A", fmt.Sprintf("%04d.test.com. 60 IN A 1.2.3.4", i))
		if err != nil {
			t.Error(err)
		}
		cache.Add(msg)
	}

	if len(cache.Cache) != 100 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}

	// Shouldnt flush any entries
	cache.Flush()
	if len(cache.Cache) != 100 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}

	// Jump forward time
	now = now.Add(time.Second * 100)
	// Should flush all entries
	cache.Flush()
	if len(cache.Cache) != 0 {
		t.Errorf("Invalid # cache items: %d", len(cache.Cache))
	}
}

func TestConcurrent(t *testing.T) {

	cache := New()
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		for i := 0; i < 100000; i++ {
			msg, _ := createCacheItem(fmt.Sprintf("%08d.test.com", i), "A", fmt.Sprintf("%08d.test.com. 5 IN A 1.2.3.4", i))
			cache.Add(msg)
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100000; i++ {
			q := util.CreateQuery(fmt.Sprintf("%08d.test.com", i), "A")
			cache.Get(q)
		}
		wg.Done()
	}()

	go func() {
		for {
			cache.Flush()
			time.Sleep(time.Millisecond * 500)
		}
	}()

	wg.Wait()
}

func TestAddRR(t *testing.T) {

	cache := New()
	err := cache.AddRR("abc.def.com 60 A 1.2.3.4", true)
	if err != nil {
		t.Error(err)
	}

	q := util.CreateQuery("abc.def.com.", "A")

	_, found := cache.Get(q)

	if found == false {
		t.Errorf("AddRR :: not found")
	}
}

func TestGetName(t *testing.T) {
	cache := New()
	err := cache.AddRR("abc.def.com 60 A 1.2.3.4", true)
	if err != nil {
		t.Error(err)
	}

	_, found := cache.GetName("abc.def.com.", "A")

	if found == false {
		t.Errorf("GetName:: not found")
	}
}
