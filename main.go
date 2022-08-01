package main

import (
	"os"

	"github.com/angelini/fusion/cmd"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
