package cmd

import "flag"

type Flags struct {
	NoCache bool
	JSON    bool
}

func ParseFlags() Flags {
	f := Flags{}
	flag.BoolVar(&f.NoCache, "no-cache", false, "bypass cache and fetch fresh data")
	flag.BoolVar(&f.JSON, "json", false, "output raw JSON instead of formatted output")
	flag.Parse()
	return f
}
