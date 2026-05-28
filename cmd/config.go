package cmd

import (
	"strings"

	ppconfig "github.com/pb33f/doctor/printingpress/config"
	"github.com/spf13/cobra"
)

func applyConfigToRootOptions(cmd *cobra.Command, opts *rootOptions, fileConfig *ppconfig.File) {
	if cmd == nil || opts == nil || fileConfig == nil {
		return
	}

	applyStringFlag(cmd, "output", &opts.outputDir, fileConfig.Output)
	applyStringFlag(cmd, "title", &opts.title, fileConfig.Title)
	applyStringFlag(cmd, "base-url", &opts.baseURL, fileConfig.BaseURL)
	applyStringFlag(cmd, "base-path", &opts.basePath, fileConfig.BasePath)
	applyStringFlag(cmd, "theme", &opts.theme, fileConfig.Theme)
	applyStringFlag(cmd, "build-mode", &opts.buildMode, fileConfig.Build.Mode)
	applyIntFlag(cmd, "max-pools", &opts.maxPools, fileConfig.Build.MaxPools)
	applyIntFlag(cmd, "workers-per-pool", &opts.workersPerPool, fileConfig.Build.WorkersPerPool)
	applyIntFlag(cmd, "max-pattern-repeat-budget", &opts.maxPatternRepeatBudget, fileConfig.Build.MaxPatternRepeatBudget)
	applyIntFlag(cmd, "max-generated-string-bytes", &opts.maxGeneratedStringBytes, fileConfig.Build.MaxGeneratedStringBytes)
	applyIntFlag(cmd, "max-generated-mock-bytes", &opts.maxGeneratedMockBytes, fileConfig.Build.MaxGeneratedMockBytes)
	applyInt64Flag(cmd, "llm-aggregate-spec-size-threshold-bytes", &opts.llmAggregateSpecSizeThresholdBytes, fileConfig.Build.LLMAggregateSpecSizeThresholdBytes)
	applyInt64Flag(cmd, "llm-max-aggregate-file-bytes", &opts.llmMaxAggregateFileBytes, fileConfig.Build.LLMMaxAggregateFileBytes)
	applyStringFlag(cmd, "llm-generate-monoliths", &opts.llmGenerateMonoliths, fileConfig.Build.LLMGenerateMonoliths)
	applyBoolFlag(cmd, "disable-skipped-rendering", &opts.disableSkippedRendering, fileConfig.Build.DisableSkippedRendering)
	applyStringFlag(cmd, "footer-url", &opts.footerURL, fileConfig.Footer.URL)
	applyStringFlag(cmd, "footer-link-title", &opts.footerLinkTitle, fileConfig.Footer.LinkTitle)
	applyStringFlag(cmd, "footer-content", &opts.footerContent, fileConfig.Footer.Content)
	applyBoolFlag(cmd, "no-logo", &opts.noLogo, fileConfig.NoLogo)
	applyBoolFlag(cmd, "disable-export", &opts.disableExport, fileConfig.DisableExport)
	applyBoolFlag(cmd, "no-html", &opts.noHTML, fileConfig.NoHTML)
	applyBoolFlag(cmd, "no-llm", &opts.noLLM, fileConfig.NoLLM)
	applyBoolFlag(cmd, "no-json", &opts.noJSON, fileConfig.NoJSON)
	applyBoolFlag(cmd, "publish", &opts.publish, fileConfig.Publish)
	applyBoolFlag(cmd, "serve", &opts.serve, fileConfig.Serve)
	applyBoolFlag(cmd, "debug", &opts.debug, fileConfig.Debug)
	applyBoolFlag(cmd, "metrics", &opts.metrics, fileConfig.Metrics)
	applyIntFlag(cmd, "port", &opts.port, fileConfig.Port)

	if !cmd.Flags().Changed("title") {
		opts.description = strings.TrimSpace(fileConfig.Description)
	}
	if fileConfig.Footer.Enabled != nil && !cmd.Flags().Changed("no-footer") {
		opts.noFooter = !*fileConfig.Footer.Enabled
	}
}

func applyStringFlag(cmd *cobra.Command, name string, dest *string, value string) {
	if dest == nil || strings.TrimSpace(value) == "" || cmd.Flags().Changed(name) {
		return
	}
	*dest = value
}

func applyBoolFlag(cmd *cobra.Command, name string, dest *bool, value bool) {
	if dest == nil || !value || cmd.Flags().Changed(name) {
		return
	}
	*dest = true
}

func applyIntFlag(cmd *cobra.Command, name string, dest *int, value int) {
	if dest == nil || value == 0 || cmd.Flags().Changed(name) {
		return
	}
	*dest = value
}

func applyInt64Flag(cmd *cobra.Command, name string, dest *int64, value int64) {
	if dest == nil || value == 0 || cmd.Flags().Changed(name) {
		return
	}
	*dest = value
}
