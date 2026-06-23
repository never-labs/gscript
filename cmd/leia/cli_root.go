package main

import (
	"errors"
	"os"
	"path/filepath"
)

func findCLIRepoRootFromCWD() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "cmd", "leia", "commands.go")) {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", errors.New("could not find Leia repository root")
		}
		dir = next
	}
}
