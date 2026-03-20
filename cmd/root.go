package cmd

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
)

type Flags struct {
	NoCache   bool
	JSON      bool
	Setup     bool
	Debug     bool
	Benchmark bool
	Verbose   bool
	Version   bool
	Update    bool
}

// version is set by goreleaser ldflags. Falls back to BuildInfo.
var version = "dev"

func GetVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

func ParseFlags() Flags {
	f := Flags{}
	flag.BoolVar(&f.NoCache, "no-cache", false, "bypass cache and fetch fresh data")
	flag.BoolVar(&f.JSON, "json", false, "output raw JSON instead of formatted output")
	flag.BoolVar(&f.Setup, "setup", false, "re-run interactive setup")
	flag.BoolVar(&f.Debug, "debug", false, "print debug info about data sources")
	flag.BoolVar(&f.Benchmark, "benchmark", false, "fetch all data with no cache and print timings")
	flag.BoolVar(&f.Verbose, "verbose", false, "show detailed fetch logs (use with --benchmark)")
	flag.BoolVar(&f.Version, "version", false, "print version and exit")
	flag.BoolVar(&f.Update, "update", false, "update to the latest release")
	flag.Parse()

	if f.Version {
		fmt.Println(GetVersion())
		os.Exit(0)
	}

	if f.Benchmark {
		f.NoCache = true
	}
	return f
}
