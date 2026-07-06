// Command gsinutshell is a health-check utility for the GSI indexing service.
// It reads the logs/stats captured in a cbcollect_info bundle and prints a
// summarised health report, flagging outliers per the GSI Nutshell spec.
//
// Usage:
//
//	gsinutshell [-window N] <cbcollect_dir>
package main

import (
	"flag"
	"fmt"
	"os"

	"gsinutshell/analyze"
	"gsinutshell/loader"
	"gsinutshell/report"
)

func main() {
	window := flag.Int("window", loader.DefaultWindow, "number of recent samples to retain per stat series")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-window N] <cbcollect_dir>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	dir := flag.Arg(0)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "error: %q is not a directory\n", dir)
		os.Exit(1)
	}

	m, err := loader.Load(dir, *window)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	r := analyze.Analyze(m)
	report.WriteText(os.Stdout, m, r)
}
