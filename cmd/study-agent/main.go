package main

import (
	"os"

	"github.com/studyforge/study-agent/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
