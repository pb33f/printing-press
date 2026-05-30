package cmd

import (
	"context"

	ppserve "github.com/pb33f/doctor/printingpress/serve"
)

type staticServerOptions = ppserve.Config

func serveOutputDir(addr string, opts staticServerOptions) error {
	return opts.ListenAndServe(context.Background(), addr)
}
