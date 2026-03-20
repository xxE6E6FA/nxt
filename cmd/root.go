package cmd

import "flag"

type Flags struct {
	NoCache bool
	JSON    bool
	Setup   bool
	Debug   bool
}

func ParseFlags() Flags {
	f := Flags{}
	flag.BoolVar(&f.NoCache, "no-cache", false, "bypass cache and fetch fresh data")
	flag.BoolVar(&f.JSON, "json", false, "output raw JSON instead of formatted output")
	flag.BoolVar(&f.Setup, "setup", false, "re-run interactive setup")
	flag.BoolVar(&f.Debug, "debug", false, "print debug info about data sources")
	flag.Parse()
	return f
}
