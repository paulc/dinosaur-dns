package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

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

func URLReader(url string, f func(s string) error) error {
	r, err := UrlOpen(url)
	if err != nil {
		return err
	}
	defer r.Close()

	return LineReader(r, f)
}

/*
func LineReader(url string, stripWS bool, stripComment string, f func(s string) error) error {

	file, err := Urlopen(url)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		line := scanner.Text()

		if stripWS {
			line = strings.Trim(line, " ")
			if len(line) == 0 {
				continue
			}
		}

		if len(stripComment) > 0 {
			if strings.HasPrefix(line, stripComment) {
				continue
			}
		}

		if err := f(line); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
*/
