package cmd

import (
	"fmt"
	"image/color"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/pb33f/doctor/printingpress"
	ppconfig "github.com/pb33f/doctor/printingpress/config"
	ppmodel "github.com/pb33f/doctor/printingpress/model"
	"github.com/pb33f/doctor/terminal"
	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Modified  bool
}

type application struct {
	info       BuildInfo
	stdout     io.Writer
	stderr     io.Writer
	httpClient *http.Client
	serveFn    func(addr string, opts staticServerOptions) error
}

type rootOptions struct {
	outputDir                          string
	title                              string
	catalogTitle                       string
	description                        string
	baseURL                            string
	basePath                           string
	theme                              string
	configPath                         string
	buildMode                          string
	maxPools                           int
	workersPerPool                     int
	maxPatternRepeatBudget             int
	maxGeneratedStringBytes            int
	maxGeneratedMockBytes              int
	llmAggregateSpecSizeThresholdBytes int64
	llmMaxAggregateFileBytes           int64
	llmGenerateMonoliths               string
	disableSkippedRendering            bool
	footerURL                          string
	footerLinkTitle                    string
	footerContent                      string
	vacuumReport                       string
	vacuumReportStdin                  bool
	noLogo                             bool
	noFooter                           bool
	disableExport                      bool
	noHTML                             bool
	noLLM                              bool
	noJSON                             bool
	publish                            bool
	serve                              bool
	port                               int
	debug                              bool
	metrics                            bool
}

type sourceInput struct {
	specBytes []byte
	basePath  string
	specPath  string
}

const commandName = "ppress"

type cliError struct {
	message string
	hint    string
	detail  string
}

func (e *cliError) Error() string {
	if e == nil {
		return ""
	}
	if e.detail != "" {
		return e.message + ": " + e.detail
	}
	return e.message
}

func Execute(version, commit, date string) {
	app := newApplication(resolveBuildInfo(version, commit, date))
	if err := app.newRootCommand().Execute(); err != nil {
		app.renderCommandError(err, terminal.PaletteForPrintingPressArgs(os.Args[1:]))
		os.Exit(1)
	}
}

func newApplication(info BuildInfo) *application {
	return &application{
		info:   info,
		stdout: os.Stdout,
		stderr: os.Stderr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		serveFn: serveOutputDir,
	}
}

func (a *application) newRootCommand() *cobra.Command {
	opts := &rootOptions{
		theme: string(terminal.ThemeDark),
		port:  9090,
	}

	cmd := &cobra.Command{
		Use:           commandName,
		Short:         "Print world class, fast, modern and LLM ready OpenAPI docs with the pb33f printing press",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runRoot(cmd, args, opts)
		},
	}

	cmd.AddCommand(a.newVersionCommand())
	cmd.Flags().StringVarP(&opts.outputDir, "output", "o", "", "Output directory for rendered docs")
	cmd.Flags().StringVar(&opts.title, "title", "", "Override the API title")
	cmd.Flags().StringVar(&opts.catalogTitle, "catalog-title", "", "Override the API catalog title")
	cmd.Flags().StringVar(&opts.configPath, "config", "", "Path to a printing-press.yaml config file")
	cmd.Flags().StringVar(&opts.baseURL, "base-url", "", "Base URL to use in generated HTML")
	cmd.Flags().StringVar(&opts.basePath, "base-path", "", "Base path for resolving local file references")
	cmd.Flags().StringVar(&opts.buildMode, "build-mode", "", "Aggregate build mode: full, fast, or watch")
	cmd.Flags().IntVar(&opts.maxPools, "max-pools", 0, "Aggregate max concurrent render pools")
	cmd.Flags().IntVar(&opts.workersPerPool, "workers-per-pool", 0, "Aggregate core budget per render pool")
	cmd.Flags().IntVar(&opts.maxPatternRepeatBudget, "max-pattern-repeat-budget", 0, "Maximum regex repeat budget for generated mock strings")
	cmd.Flags().IntVar(&opts.maxGeneratedStringBytes, "max-generated-string-bytes", 0, "Maximum bytes for each generated mock string")
	cmd.Flags().IntVar(&opts.maxGeneratedMockBytes, "max-generated-mock-bytes", 0, "Maximum bytes for each serialized generated mock payload")
	cmd.Flags().Int64Var(&opts.llmAggregateSpecSizeThresholdBytes, "llm-aggregate-spec-size-threshold-bytes", 0, "Root spec byte threshold for generating monolithic LLM aggregate files")
	cmd.Flags().Int64Var(&opts.llmMaxAggregateFileBytes, "llm-max-aggregate-file-bytes", 0, "Target maximum bytes for each sharded LLM aggregate file")
	cmd.Flags().StringVar(&opts.llmGenerateMonoliths, "llm-generate-monoliths", "", "LLM monolithic aggregate mode: auto, always, or never")
	cmd.Flags().BoolVar(&opts.disableSkippedRendering, "disable-skipped-rendering", false, "Hide skipped-render warnings from aggregate catalog pages")
	cmd.Flags().StringVar(&opts.footerURL, "footer-url", "", "Footer link URL for generated HTML")
	cmd.Flags().StringVar(&opts.footerLinkTitle, "footer-link-title", "", "Footer link text/title for generated HTML")
	cmd.Flags().StringVar(&opts.footerContent, "footer-content", "", "Footer trailing content text for generated HTML")
	cmd.Flags().StringVar(&opts.vacuumReport, "vacuum-report", "", "Path to a vacuum sealed report to render as lint diagnostics")
	cmd.Flags().BoolVarP(&opts.vacuumReportStdin, "stdin", "i", false, "Read a vacuum sealed report from stdin for lint diagnostics")
	cmd.Flags().StringVar(&opts.theme, "theme", string(terminal.ThemeDark), "Terminal theme: dark, roger, or tektronix")
	cmd.Flags().BoolVarP(&opts.noLogo, "no-logo", "b", false, "Disable the pb33f banner")
	cmd.Flags().BoolVar(&opts.noFooter, "no-footer", false, "Disable the generated HTML footer")
	cmd.Flags().BoolVar(&opts.disableExport, "disable-export", false, "Disable local preview archive export controls")
	cmd.Flags().BoolVar(&opts.noHTML, "no-html", false, "Skip HTML output")
	cmd.Flags().BoolVar(&opts.noLLM, "no-llm", false, "Skip LLM output")
	cmd.Flags().BoolVar(&opts.noJSON, "no-json", false, "Skip JSON artifact output")
	cmd.Flags().BoolVar(&opts.publish, "publish", false, "Build hosted/served HTML assets without starting a local server")
	cmd.Flags().BoolVar(&opts.serve, "serve", false, "Serve the rendered output after building")
	cmd.Flags().IntVar(&opts.port, "port", 9090, "Port to use with --serve")
	cmd.Flags().BoolVar(&opts.debug, "debug", false, "Disable the progress bar and stream build logs live")
	cmd.Flags().BoolVar(&opts.metrics, "metrics", false, "Show live aggregate runtime metrics while rendering")

	cmd.SetOut(a.stdout)
	cmd.SetErr(a.stderr)
	return cmd
}

func (a *application) newVersionCommand() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print the current version of printing-press",
		Example: commandName + " version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !verbose {
				fmt.Fprintln(a.stdout, a.info.Version)
				return nil
			}

			fmt.Fprintf(a.stdout, "version: %s\n", a.info.Version)
			fmt.Fprintf(a.stdout, "commit: %s\n", a.info.Commit)
			fmt.Fprintf(a.stdout, "date: %s\n", a.info.Date)
			if a.info.GoVersion != "" {
				fmt.Fprintf(a.stdout, "go: %s\n", a.info.GoVersion)
			}
			if a.info.Modified {
				fmt.Fprintln(a.stdout, "modified: true")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print extended build information")
	return cmd
}

func (a *application) runRoot(cmd *cobra.Command, args []string, opts *rootOptions) error {
	fileConfig, err := ppconfig.Load(opts.configPath, firstArg(args))
	if err != nil {
		return &cliError{
			message: "unable to load configuration",
			hint:    "Use --config /path/to/printing-press.yaml or place printing-press.yaml in the current or target directory.",
			detail:  err.Error(),
		}
	}
	applyConfigToRootOptions(cmd, opts, fileConfig)

	theme, err := terminal.ParsePrintingPressTheme(opts.theme)
	if err != nil {
		return &cliError{
			message: "invalid terminal theme",
			hint:    "Use --theme dark, --theme roger, or --theme tektronix.",
			detail:  err.Error(),
		}
	}
	palette := terminal.PaletteForTheme(theme)

	if len(args) > 1 {
		return &cliError{
			message: "expected exactly one spec path, directory, or URL",
			hint:    fmt.Sprintf("Try '%s ./openapi.yaml', '%s ./apis', or '%s --serve ./openapi.yaml'.", commandName, commandName, commandName),
			detail:  fmt.Sprintf("received %d arguments", len(args)),
		}
	}

	inputArg, inputSet := resolveBuildInput(args, fileConfig)
	if !inputSet {
		if hasDeveloperDiagnosticsInput(opts) {
			return &cliError{
				message: "expected exactly one spec path, directory, or URL",
				hint:    "--stdin reads the vacuum report; pass the OpenAPI spec as the positional argument.",
			}
		}
		a.printWelcome(opts, palette)
		return nil
	}

	return a.runBuild(inputArg, opts, palette, fileConfig, cmd.InOrStdin())
}

func (a *application) runBuild(specArg string, opts *rootOptions, palette terminal.Palette, fileConfig *ppconfig.File, diagnosticsInput io.Reader) (err error) {
	if !isRemoteInput(specArg) {
		absPath, absErr := filepath.Abs(specArg)
		if absErr == nil {
			if info, statErr := os.Stat(absPath); statErr == nil && info.IsDir() {
				if hasDeveloperDiagnosticsInput(opts) {
					return &cliError{
						message: "lint diagnostics are only supported for single-spec builds",
						hint:    "Pass a single OpenAPI file or URL with --stdin or --vacuum-report.",
					}
				}
				return a.runAggregateBuild(absPath, opts, palette, fileConfig)
			}
		}
	}
	return a.runSingleSpecBuild(specArg, opts, palette, diagnosticsInput)
}

func (a *application) runSingleSpecBuild(specArg string, opts *rootOptions, palette terminal.Palette, diagnosticsInput io.Reader) (err error) {
	if opts.noHTML && opts.noLLM && opts.noJSON {
		return &cliError{
			message: "all output types are disabled",
			hint:    "Leave at least one of HTML, LLM, or JSON enabled.",
		}
	}

	buildStart := time.Now()
	renderMode := terminal.SelectActivityRenderMode(a.stderr, opts.debug)
	loggerSession := terminal.ConfigureBuildLogger(a.stderr, palette, renderMode)
	defer func() {
		loggerSession.Finish(err)
	}()

	a.printBannerIfEnabled(opts, palette)

	renderer := terminal.NewActivityRenderer(renderMode, a.stderr, palette, buildOutputStageCount(opts, false), loggerSession.Logger)
	defer renderer.Close()

	source, err := a.loadSource(specArg, opts.basePath)
	if err != nil {
		return &cliError{
			message: "unable to load specification input",
			hint:    "Pass a local OpenAPI file path or an http(s) URL.",
			detail:  err.Error(),
		}
	}

	outputDir, err := normalizeOutputDir(opts.outputDir)
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

	developerMode, lintResults, err := resolveDeveloperDiagnostics(opts, diagnosticsInput)
	if err != nil {
		return &cliError{
			message: "unable to load lint report",
			hint:    "Pass a vacuum sealed report generated by 'vacuum report'.",
			detail:  err.Error(),
		}
	}

	footer := buildFooterConfig(opts)
	pp, err := printingpress.CreatePrintingPressFromBytes(source.specBytes, &printingpress.PrintingPressConfig{
		Title:                              opts.title,
		BaseURL:                            opts.baseURL,
		BasePath:                           source.basePath,
		SpecPath:                           source.specPath,
		OutputDir:                          outputDir,
		AssetMode:                          assetMode,
		DeveloperMode:                      developerMode,
		ArchiveExportURL:                   archiveExportURLForServe(opts),
		LintResults:                        lintResults,
		Footer:                             footer,
		MaxPatternRepeatBudget:             opts.maxPatternRepeatBudget,
		MaxGeneratedStringBytes:            opts.maxGeneratedStringBytes,
		MaxGeneratedMockBytes:              opts.maxGeneratedMockBytes,
		LLMAggregateSpecSizeThresholdBytes: opts.llmAggregateSpecSizeThresholdBytes,
		LLMMaxAggregateFileBytes:           opts.llmMaxAggregateFileBytes,
		LLMGenerateMonoliths:               opts.llmGenerateMonoliths,
	})
	if err != nil {
		return &cliError{
			message: "unable to create printing press",
			detail:  err.Error(),
		}
	}

	var htmlStats *printingpress.PressStatistics
	var llmStats *printingpress.PressStatistics
	var site *ppmodel.Site

	if !opts.noHTML {
		htmlStats, err = terminal.RunWithActivity(pp, renderer, pp.PrintHTML)
		if err != nil {
			return &cliError{message: "html render failed", detail: err.Error()}
		}
	}

	if !opts.noLLM {
		llmStats, err = terminal.RunWithActivity(pp, renderer, pp.PrintLLM)
		if err != nil {
			return &cliError{message: "llm render failed", detail: err.Error()}
		}
	}

	if !opts.noJSON {
		if htmlStats == nil && llmStats == nil {
			site, err = terminal.RunWithActivity(pp, renderer, pp.PressModel)
			if err != nil {
				return &cliError{message: "model build failed", detail: err.Error()}
			}
		} else {
			site, err = pp.PressModel()
			if err != nil {
				return &cliError{message: "model build failed", detail: err.Error()}
			}
		}
		jsonStart := time.Now()
		renderer.UpdateManual("json", "writing json artifacts", "running", 0.2, 0, nil)
		if err := printingpress.PrintJSONArtifacts(site, ""); err != nil {
			renderer.UpdateManual("json", "json artifact write failed", "failed", 0, time.Since(jsonStart), err)
			return &cliError{message: "json artifact write failed", detail: err.Error()}
		}
		jsonDuration := time.Since(jsonStart)
		renderer.UpdateManual("json", "json artifacts complete", "completed", 1, jsonDuration, nil)
	}

	if site == nil {
		site, err = pp.PressModel()
		if err != nil {
			return &cliError{message: "model build failed", detail: err.Error()}
		}
	}

	fileCount, totalBytes, err := terminal.ScanOutputDir(site.OutputDir)
	if err != nil {
		return &cliError{message: "unable to scan output directory", detail: err.Error()}
	}

	renderer.Close()
	terminal.PrintSummary(a.stdout, palette, site, htmlStats, llmStats, time.Since(buildStart), fileCount, totalBytes)

	if opts.serve {
		serveOpts := staticServerOptions{
			Dir:           site.OutputDir,
			BaseURL:       site.BaseURL,
			DisableExport: opts.disableExport,
		}
		if !opts.disableExport {
			archiveDirs, err := renderServeArchiveDirs(*source, opts, lintResults, footer)
			if err != nil {
				return &cliError{message: "unable to render served archive export", detail: err.Error()}
			}
			defer archiveDirs.Cleanup()
			if archiveDirs != nil {
				serveOpts.ArchiveDir = archiveDirs.Plain
				serveOpts.DiagnosticsArchiveDir = archiveDirs.Diagnostics
				serveOpts.LLMArchiveDir = archiveDirs.LLM
				serveOpts.DiagnosticsLLMArchiveDir = archiveDirs.DiagnosticsLLM
			}
		}
		fmt.Fprintf(a.stdout, "serving http://127.0.0.1:%d from %s\n", opts.port, site.OutputDir)
		if err := a.serveFn(fmt.Sprintf(":%d", opts.port), serveOpts); err != nil {
			return &cliError{message: "unable to serve rendered output", detail: err.Error()}
		}
	}

	return nil
}

func (a *application) printBannerIfEnabled(opts *rootOptions, palette terminal.Palette) {
	if opts.noLogo {
		return
	}
	terminal.PrintBanner(terminal.BannerOptions{
		Writer:      a.stdout,
		Palette:     palette,
		ProductName: "printing-press",
		ProductURL:  "https://pb33f.io/printing-press/",
		Version:     a.info.Version,
		Date:        a.info.Date,
	})
}

func (a *application) printWelcome(opts *rootOptions, palette terminal.Palette) {
	a.printBannerIfEnabled(opts, palette)

	title := styleWithForeground(palette.Primary).Bold(true)
	accent := styleWithForeground(palette.Secondary).Bold(true)
	muted := styleWithForeground(palette.Muted)

	fmt.Fprintln(a.stdout, title.Render(fmt.Sprintf(">> Welcome! To render docs, try '%s ./openapi.yaml'", commandName)))
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, muted.Render("Default outputs:"))
	fmt.Fprintln(a.stdout, "  > html site")
	fmt.Fprintln(a.stdout, "  > llm docs")
	fmt.Fprintln(a.stdout, "  > json bundle + artifacts")
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, muted.Render("Examples:"))
	fmt.Fprintln(a.stdout, "  "+accent.Render(commandName+" ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render(commandName+" ./apis"))
	fmt.Fprintln(a.stdout, "  "+accent.Render(commandName+" --debug ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render(commandName+" --publish --output ./api-docs ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render(commandName+" --serve --output ./api-docs ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render(commandName+" https://example.com/openapi.yaml"))
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, title.Render(fmt.Sprintf("To see all the options, try '%s --help'", commandName)))
	fmt.Fprintln(a.stdout)
}

func parseTheme(raw string) (terminal.ThemeName, error) {
	return terminal.ParsePrintingPressTheme(raw)
}

func paletteForArgs(args []string) terminal.Palette {
	return terminal.PaletteForPrintingPressArgs(args)
}

func buildFooterConfig(opts *rootOptions) *ppmodel.FooterConfig {
	if opts == nil {
		return nil
	}
	footerURL := strings.TrimSpace(opts.footerURL)
	footerLinkTitle := strings.TrimSpace(opts.footerLinkTitle)
	footerContent := strings.TrimSpace(opts.footerContent)
	if !opts.noFooter && footerURL == "" && footerLinkTitle == "" && footerContent == "" {
		return nil
	}
	return &ppmodel.FooterConfig{
		Disabled:  opts.noFooter,
		URL:       footerURL,
		LinkTitle: footerLinkTitle,
		Build:     footerContent,
	}
}

func buildOutputStageCount(opts *rootOptions, diagnostics bool) int {
	if opts == nil {
		return 1
	}
	return terminal.BuildStageCount(terminal.OutputSelection{
		HTML:        !opts.noHTML,
		LLM:         !opts.noLLM,
		JSON:        !opts.noJSON,
		Diagnostics: diagnostics,
	})
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func resolveBuildInput(args []string, fileConfig *ppconfig.File) (string, bool) {
	if len(args) > 0 {
		return args[0], true
	}
	if fileConfig == nil {
		return "", false
	}
	if strings.TrimSpace(fileConfig.Scan.Root) != "" {
		return strings.TrimSpace(fileConfig.Scan.Root), true
	}
	return "", false
}

func normalizeOutputDir(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	return abs, nil
}

func (a *application) loadSource(specArg, basePathFlag string) (*sourceInput, error) {
	if isRemoteInput(specArg) {
		return a.loadRemoteSource(specArg, basePathFlag)
	}
	return a.loadLocalSource(specArg, basePathFlag)
}

func isRemoteInput(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (a *application) loadRemoteSource(rawURL, basePathFlag string) (*sourceInput, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "printing-press")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download spec: unexpected status %s", resp.Status)
	}

	specBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read downloaded spec: %w", err)
	}

	basePath, err := normalizeBasePath(basePathFlag)
	if err != nil {
		return nil, err
	}

	return &sourceInput{specBytes: specBytes, basePath: basePath, specPath: rawURL}, nil
}

func (a *application) loadLocalSource(specPath, basePathFlag string) (*sourceInput, error) {
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return nil, fmt.Errorf("resolve spec path: %w", err)
	}

	specBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read spec %q: %w", specPath, err)
	}

	basePath := basePathFlag
	if basePath == "" {
		basePath = filepath.Dir(absPath)
	}

	resolvedBasePath, err := normalizeBasePath(basePath)
	if err != nil {
		return nil, err
	}

	return &sourceInput{specBytes: specBytes, basePath: resolvedBasePath, specPath: absPath}, nil
}

func normalizeBasePath(basePath string) (string, error) {
	if basePath == "" {
		return "", nil
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("resolve base path: %w", err)
	}
	return abs, nil
}

func (a *application) renderCommandError(err error, palette terminal.Palette) {
	prefix := styleWithForeground(palette.Breaking).Bold(true).Render("✗")
	headlineStyle := styleWithForeground(palette.Breaking).Bold(true)
	detailStyle := styleWithForeground(palette.Muted)
	hintStyle := styleWithForeground(palette.Primary)

	headline := err.Error()
	detail := ""
	hint := ""
	if cliErr, ok := err.(*cliError); ok {
		headline = cliErr.message
		detail = cliErr.detail
		hint = cliErr.hint
	}

	fmt.Fprintf(a.stderr, "%s %s\n", prefix, headlineStyle.Render(headline))
	if detail != "" {
		fmt.Fprintf(a.stderr, "  %s\n", detailStyle.Render(detail))
	}
	if hint != "" {
		fmt.Fprintf(a.stderr, "  %s\n", hintStyle.Render(hint))
	}
	fmt.Fprintln(a.stderr)
}

func styleWithForeground(c color.Color) lipgloss.Style {
	if c == nil {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(c)
}
