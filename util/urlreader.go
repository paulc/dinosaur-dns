package util

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// Open file or http URL
func UrlOpen(arg string) (io.ReadCloser, error) {
	target, err := url.Parse(arg)
	if err != nil {
		return nil, fmt.Errorf("Couldnt parse URL <%s> %s", arg, err)
	}

	if target.Scheme == "" {
		return os.Open(arg)
	} else if target.Scheme == "http" || target.Scheme == "https" {
		resp, err := http.Get(arg)
		if err != nil {
			return nil, fmt.Errorf("Error fetching URL <%s>: %s", arg, err)
		}
		return resp.Body, nil
	} else if target.Scheme == "file" {
		return os.Open(target.Path)
	} else {
		return nil, fmt.Errorf("Invalid URL scheme: %s (http/https/file supported)", arg)
	}
}

// Call f for each line in io.Reader
func LineReader(r io.Reader, f func(s string) error) (count int, err error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := f(scanner.Text()); err != nil {
			return count, fmt.Errorf("Error calling line function: %s", err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("Scanner Error: %s", err)
	}
	return count, nil
}

// Open file/url and run f for each line
func URLReader(url string, f func(s string) error) (int, error) {
	r, err := UrlOpen(url)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	return LineReader(r, f)
}
