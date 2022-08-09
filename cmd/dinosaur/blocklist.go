package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/miekg/dns"
	"github.com/paulc/aaaa_proxy/block"
)

func addBlocklistEntry(blocklist block.BlockList, entry string) {
	split := strings.Split(entry, ":")
	if len(split) == 1 {
		blocklist.AddName(split[0], dns.TypeANY)
	} else if len(split) == 2 {
		qtype, ok := dns.StringToType[split[1]]
		if !ok {
			log.Fatalf("Invalid qtype: %s:%s", split[0], split[1])
		}
		blocklist.AddName(split[0], qtype)
	} else {
		log.Fatalf("Invalid blocklist entry: %s", strings.Join(split, ":"))
	}
}

func addBlocklistFromFile(blocklist block.BlockList, f string) {
	file, err := os.Open(f)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " ")
		// Ignore blank lines or comments
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		addBlocklistEntry(blocklist, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func addBlocklistFromHostsFile(blocklist block.BlockList, f string) {
	file, err := os.Open(f)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " ")
		// Ignore blank lines or comments
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		addBlocklistEntry(blocklist, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
