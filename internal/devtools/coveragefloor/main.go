package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

var coveragePattern = regexp.MustCompile(`coverage: ([0-9.]+)%`)

func main() {
	var floor float64
	var goBinary string
	flag.Float64Var(&floor, "floor", 0, "minimum coverage percentage required for each package")
	flag.StringVar(&goBinary, "go", "go", "go binary to execute")
	flag.Parse()

	packages := flag.Args()
	if floor <= 0 {
		fmt.Fprintln(os.Stderr, "coverage floor must be greater than zero")
		os.Exit(2)
	}
	if len(packages) == 0 {
		fmt.Fprintln(os.Stderr, "at least one package is required")
		os.Exit(2)
	}

	status := 0
	for _, pkg := range packages {
		output, err := runCoverage(goBinary, pkg)
		fmt.Print(output)
		if len(output) > 0 && output[len(output)-1] != '\n' {
			fmt.Println()
		}
		if err != nil {
			status = 1
			continue
		}

		coverage, err := parseCoverage(output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: could not parse coverage for %s: %v\n", pkg, err)
			status = 1
			continue
		}
		if coverage < floor {
			fmt.Fprintf(os.Stderr, "FAIL: %s coverage %.1f%% below floor %.1f%%\n", pkg, coverage, floor)
			status = 1
			continue
		}
		fmt.Printf("OK: %s coverage %.1f%% (floor %.1f%%)\n", pkg, coverage, floor)
	}
	os.Exit(status)
}

func runCoverage(goBinary, pkg string) (string, error) {
	cmd := exec.Command(goBinary, "test", pkg, "-count=1", "-cover")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.String(), err
}

func parseCoverage(output string) (float64, error) {
	match := coveragePattern.FindStringSubmatch(output)
	if match == nil {
		return 0, fmt.Errorf("missing coverage line")
	}
	return strconv.ParseFloat(match[1], 64)
}
