package main

import (
	"os"

	"github.com/ctoth/claudio/internal/cli"
)

func main() {
	// Create CLI instance and run with system arguments and I/O
	c := cli.NewCLI()
	exitCode := c.Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
