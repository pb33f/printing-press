package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pb33f/doctor/printingpress"
	ppmodel "github.com/pb33f/doctor/printingpress/model"
	"github.com/pb33f/doctor/terminal"
)

func (a *application) runAggregateBuild(scanRoot string, opts *rootOptions, palette terminal.Palette, fileConfig *printingPressConfigFile) (err error) {
	if opts.noHTML && opts.noLLM && opts.noJSON {
		return &cliError{
			message: "all output types are disabled",
			hint:    "Leave at least one of HTML, LLM, or JSON enabled.",
		}
	}

	buildStart := time.Now()
	renderMode := selectActivityRenderMode(a.stderr, opts.debug)
	loggerSession := a.configureBuildLogger(palette, renderMode)
	defer func() {
		loggerSession.finish(err)
	}()

	a.printBannerIfEnabled(opts, palette)

	scanRenderer := newActivityRenderer(renderMode, a.stderr, palette, 1, loggerSession.logger)
	defer scanRenderer.Close()

	outputDir, err := normalizeAggregateOutputDir(opts.outputDir)
	if err != nil {
		return &cliError{
			message: "unable to resolve output directory",
			detail:  err.Error(),
		}
	}

	assetMode := printingpress.HTMLAssetModePortable
	if opts.publish || opts.serve {
		assetMode = printingpress.HTMLAssetModeServed
	}

	ap, err := printingpress.CreateAggregatePrintingPressFromPath(scanRoot, buildAggregateConfig(scanRoot, outputDir, assetMode, opts, fileConfig))
	if err != nil {
		return &cliError{
			message: "unable to create aggregate printing press",
			detail:  err.Error(),
		}
	}

	catalog, err := runAggregateCatalogStage(scanRenderer, ap)
	if err != nil {
		return &cliError{message: "catalog discovery failed", detail: err.Error()}
	}
	scanRenderer.Close()

	poolRenderer := newAggregatePoolRenderer(renderMode, a.stderr, palette, loggerSession.logger)
	defer poolRenderer.Close()

	stats, err := ap.PrintSelectedOutputs(printingpress.AggregateRenderOptions{
		HTML: !opts.noHTML,
		LLM:  !opts.noLLM,
		JSON: !opts.noJSON,
		ProgressReporter: printingpress.AggregateProgressReporterFunc(func(update printingpress.AggregateProgressUpdate) {
			poolRenderer.report(update)
		}),
	})
	if err != nil {
		return &cliError{message: "aggregate render failed", detail: err.Error()}
	}

	fileCount, totalBytes, err := scanOutputDir(catalog.OutputDir)
	if err != nil {
		return &cliError{message: "unable to scan output directory", detail: err.Error()}
	}

	poolRenderer.Close()
	a.printAggregateSummary(palette, catalog, stats, nil, nil, time.Since(buildStart), fileCount, totalBytes)

	if opts.serve {
		fmt.Fprintf(a.stdout, "serving http://127.0.0.1:%d from %s\n", opts.port, catalog.OutputDir)
		if err := a.serveFn(fmt.Sprintf(":%d", opts.port), catalog.OutputDir, opts.baseURL); err != nil {
			return &cliError{message: "unable to serve rendered output", detail: err.Error()}
		}
	}

	return nil
}

func buildAggregateConfig(scanRoot, outputDir, assetMode string, opts *rootOptions, fileConfig *printingPressConfigFile) *printingpress.AggregatePrintingPressConfig {
	catalogTitle := opts.title
	if override := strings.TrimSpace(opts.catalogTitle); override != "" {
		catalogTitle = override
	}

	cfg := &printingpress.AggregatePrintingPressConfig{
		Title:                   catalogTitle,
		Description:             opts.description,
		ScanRoot:                scanRoot,
		OutputDir:               outputDir,
		BaseURL:                 opts.baseURL,
		AssetMode:               assetMode,
		BuildMode:               opts.buildMode,
		MaxPools:                opts.maxPools,
		WorkersPerPool:          opts.workersPerPool,
		DisableSkippedRendering: opts.disableSkippedRendering,
		Footer:                  buildFooterConfig(opts),
	}
	if fileConfig == nil {
		return cfg
	}

	cfg.Include = append([]string(nil), fileConfig.Scan.Include...)
	cfg.IgnoreRules = append([]string(nil), fileConfig.Scan.IgnoreRules...)
	cfg.NoiseSegments = append([]string(nil), fileConfig.Grouping.NoiseSegments...)
	cfg.ServiceOverrides = toAggregateOverrides(fileConfig.Grouping.ServiceOverrides)
	cfg.DisplayNameOverrides = toAggregateOverrides(fileConfig.Grouping.DisplayNameOverrides)
	cfg.VersionOverrides = toAggregateOverrides(fileConfig.Grouping.VersionOverrides)
	cfg.StateNamespace = fileConfig.State.Namespace
	cfg.StateSQLitePath = fileConfig.State.SQLite.Path
	if cfg.MaxPools == 0 {
		cfg.MaxPools = fileConfig.Build.MaxPools
	}
	if cfg.WorkersPerPool == 0 {
		cfg.WorkersPerPool = fileConfig.Build.WorkersPerPool
	}
	if !cfg.DisableSkippedRendering {
		cfg.DisableSkippedRendering = fileConfig.Build.DisableSkippedRendering
	}
	return cfg
}

func toAggregateOverrides(configs []printingPressPathConfig) []printingpress.AggregatePathOverride {
	overrides := make([]printingpress.AggregatePathOverride, 0, len(configs))
	for _, override := range configs {
		if override.Pattern == "" || override.Value == "" {
			continue
		}
		overrides = append(overrides, printingpress.AggregatePathOverride{
			Pattern: override.Pattern,
			Value:   override.Value,
		})
	}
	return overrides
}

func runAggregateCatalogStage(renderer activityRenderer, ap *printingpress.AggregatePrintingPress) (*ppmodel.CatalogSite, error) {
	start := time.Now()
	renderer.updateManual("scan", "discovering specs", "running", 0.05, 0, nil)
	catalog, err := ap.PressModel()
	if err != nil {
		renderer.updateManual("scan", "spec discovery failed", "failed", 0, time.Since(start), err)
		return nil, err
	}
	renderer.updateManual("scan", fmt.Sprintf("discovered %d services across %d specs", len(catalog.Services), countCatalogSpecs(catalog)), "completed", 1, time.Since(start), nil)
	return catalog, nil
}

func runAggregateStage(renderer activityRenderer, stage, task string, run func() (*printingpress.AggregatePressStatistics, error)) (*printingpress.AggregatePressStatistics, error) {
	start := time.Now()
	renderer.updateManual(stage, task, "running", 0.15, 0, nil)
	stats, err := run()
	if err != nil {
		renderer.updateManual(stage, stage+" failed", "failed", 0, time.Since(start), err)
		return nil, err
	}
	renderer.updateManual(stage, stage+" complete", "completed", 1, time.Since(start), nil)
	return stats, nil
}

func countCatalogSpecs(catalog *ppmodel.CatalogSite) int {
	if catalog == nil {
		return 0
	}
	total := 0
	for _, service := range catalog.Services {
		if service == nil {
			continue
		}
		total += service.SpecCount
	}
	return total
}

func aggregateOutputDir(scanRoot string) string {
	return filepath.Join(scanRoot, "api-docs")
}

func normalizeAggregateOutputDir(raw string) (string, error) {
	if raw != "" {
		return normalizeOutputDir(raw)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return filepath.Join(cwd, "api-docs"), nil
}
