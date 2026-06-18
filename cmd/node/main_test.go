package main

import (
	"os"
	"testing"
)

func TestMainFunc(t *testing.T) {
	t.Setenv("NODE_ID", "testnode")
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	dir := t.TempDir()
	originalWd, err := os.Getwd()
	if err == nil {
		err = os.Chdir(dir)
		if err == nil {
			defer func() { _ = os.Chdir(originalWd) }()
		}
	}

	os.Args = []string{"node", "createwallet"}
	main()
}
