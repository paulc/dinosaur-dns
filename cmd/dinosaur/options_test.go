package main

import (
	"os"
	"testing"
)

func TestGetUserConfig(t *testing.T) {

	// XXX Test more flags
	os.Args = []string{"progname", "-debug"}

	user_config, err := GetUserConfig()
	if err != nil {
		t.Fatal(err)
	}

	if !user_config.Debug {
		t.Errorf("Debug: %t", user_config.Debug)
	}
}
