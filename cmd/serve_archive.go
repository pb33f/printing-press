package cmd

import (
	"context"

	ppserve "github.com/pb33f/doctor/printingpress/serve"
)

type staticServerOptions = ppserve.Config

func archiveExportURLForServe(opts *rootOptions) string {
	if opts == nil || !opts.serve || opts.disableExport {
		return ""
	}
	return ppserve.ArchiveExportPathForBaseURL(opts.baseURL)
}

func serveOutputDir(addr string, opts staticServerOptions) error {
	return opts.ListenAndServe(context.Background(), addr)
}
