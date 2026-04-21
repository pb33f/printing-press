package main

import (
	"github.com/pb33f/printing-press/cmd"
)

var version string
var commit string
var date string

func main() {
	cmd.Execute(version, commit, date)
}
