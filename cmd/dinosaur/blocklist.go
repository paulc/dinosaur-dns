package main

import (
	"bufio"
	"fmt"
	"log"
	"strings"

	"github.com/miekg/dns"
	"github.com/paulc/dinosaur/block"
)

func addBlocklistEntry(blocklist block.BlockList, entry string, default_qtype uint16) {
	split := strings.Split(entry, ":")
	if len(split) == 1 {
		blocklist.AddName(split[0], default_qtype)
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

func addBlocklistFromFile(blocklist block.BlockList, f string, default_qtype uint16) {
	file, err := Urlopen(f)
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
		addBlocklistEntry(blocklist, scanner.Text(), default_qtype)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func splitHostsEntry(entry string) (ip, domain string, err error) {
	split := strings.Split(entry, " ")
	if len(split) == 1 {
		return "", "", fmt.Errorf("Invalid hosts entry: %s", entry)
	}
	return split[0], split[1], nil
}

func addBlocklistFromHostsFile(blocklist block.BlockList, f string) {
	file, err := Urlopen(f)
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

		// Split into IP / Domain pair
		ip, domain, err := splitHostsEntry(line)
		if err != nil {
			log.Fatal(err)
		}

		// Skip unless IP is 0.0.0.0
		if ip != "0.0.0.0" || domain == "0.0.0.0" {
			continue
		}

		addBlocklistEntry(blocklist, domain, dns.TypeANY)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
