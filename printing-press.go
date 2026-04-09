package main

import (
	"time"

	"github.com/pb33f/printing-press/cmd"
)

var version string
var commit string
var date string

func main() {
	if version == "" {
		version = "latest"
	}
	if commit == "" {
		commit = "latest"
	}
	if date == "" {
		date = time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST")
	} else if parsed, err := time.Parse(time.RFC3339, date); err == nil {
		date = parsed.Format("Mon, 02 Jan 2006 15:04:05 MST")
	}

	cmd.Execute(version, commit, date)
}
