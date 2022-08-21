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
		return nil, err
	}

	if target.Scheme == "" {
		return os.Open(arg)
	} else if target.Scheme == "http" || target.Scheme == "https" {
		resp, err := http.Get(arg)
		if err != nil {
			return nil, fmt.Errorf("urlGet Error: %s", err)
		}
		return resp.Body, nil
	} else if target.Scheme == "file" {
		return os.Open(target.Path)
	} else {
		return nil, fmt.Errorf("Error: Invalid URL scheme: %s (http/https/file supported)", arg)
	}
}

// Call f for each line in io.Reader
func LineReader(r io.Reader, f func(s string) error) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := f(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// Open file/url and run f for each line
func URLReader(url string, f func(s string) error) error {
	r, err := UrlOpen(url)
	if err != nil {
		return err
	}
	defer r.Close()

	return LineReader(r, f)
}
