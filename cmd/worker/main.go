package main

import (
	"os"

	"github.com/nuriansyah/hugimethara/internal/bootstrap"
)

func main() {
	if err := bootstrap.RunWorker(); err != nil {
		os.Exit(1)
	}
}
