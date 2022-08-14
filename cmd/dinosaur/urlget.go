package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

func Urlopen(arg string) (io.ReadCloser, error) {

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
