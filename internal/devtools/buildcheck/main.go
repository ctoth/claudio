package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	var goBinary string
	var noCGO bool
	flag.StringVar(&goBinary, "go", "go", "go binary to execute")
	flag.BoolVar(&noCGO, "nocgo", false, "run build with CGO_ENABLED=0")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"./cmd/claudio"}
	}

	output := filepath.Join(os.TempDir(), "claudio-buildcheck")
	if os.PathSeparator == '\\' {
		output += ".exe"
	}

	cmdArgs := append([]string{"build", "-o", output}, args...)
	if noCGO {
		cmdArgs = append([]string{"build"}, args...)
	}

	cmd := exec.Command(goBinary, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if noCGO {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build check failed: %v\n", err)
		os.Exit(1)
	}
	if !noCGO {
		_ = os.Remove(output)
	}
}
