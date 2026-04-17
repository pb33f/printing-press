package cmd

import (
	"compress/gzip"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/pb33f/doctor/printingpress"
	ppmodel "github.com/pb33f/doctor/printingpress/model"
	"github.com/pb33f/doctor/terminal"
	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type application struct {
	info       BuildInfo
	stdout     io.Writer
	stderr     io.Writer
	httpClient *http.Client
	serveFn    func(addr, dir string) error
}

type buildLoggerSession struct {
	previous *slog.Logger
	logger   *slog.Logger
	handler  *terminal.PrettyHandler
	buffered bool
}

type rootOptions struct {
	outputDir               string
	title                   string
	description             string
	baseURL                 string
	basePath                string
	theme                   string
	configPath              string
	buildMode               string
	maxPools                int
	workersPerPool          int
	disableSkippedRendering bool
	noLogo                  bool
	noHTML                  bool
	noLLM                   bool
	noJSON                  bool
	publish                 bool
	serve                   bool
	port                    int
	debug                   bool
}

var activityRenderWaitTimeout = 2 * time.Second

type sourceInput struct {
	specBytes []byte
	basePath  string
	specPath  string
}

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
	app := newApplication(BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
	if err := app.newRootCommand().Execute(); err != nil {
		app.renderCommandError(err, paletteForArgs(os.Args[1:]))
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
		Use:           "printing-press",
		Short:         "Print world class, fast, modern and LLM ready OpenAPI docs with the pb33f printing press",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runRoot(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.outputDir, "output", "o", "", "Output directory for rendered docs")
	cmd.Flags().StringVar(&opts.title, "title", "", "Override the API title")
	cmd.Flags().StringVar(&opts.configPath, "config", "", "Path to a printing-press.yaml config file")
	cmd.Flags().StringVar(&opts.baseURL, "base-url", "", "Base URL to use in generated HTML")
	cmd.Flags().StringVar(&opts.basePath, "base-path", "", "Base path for resolving local file references")
	cmd.Flags().StringVar(&opts.buildMode, "build-mode", "", "Aggregate build mode: full, fast, or watch")
	cmd.Flags().IntVar(&opts.maxPools, "max-pools", 0, "Aggregate max concurrent render pools")
	cmd.Flags().IntVar(&opts.workersPerPool, "workers-per-pool", 0, "Aggregate core budget per render pool")
	cmd.Flags().BoolVar(&opts.disableSkippedRendering, "disable-skipped-rendering", false, "Hide skipped-render warnings from aggregate catalog pages")
	cmd.Flags().StringVar(&opts.theme, "theme", string(terminal.ThemeDark), "Terminal theme: dark, roger, or tektronix")
	cmd.Flags().BoolVarP(&opts.noLogo, "no-logo", "b", false, "Disable the pb33f banner")
	cmd.Flags().BoolVar(&opts.noHTML, "no-html", false, "Skip HTML output")
	cmd.Flags().BoolVar(&opts.noLLM, "no-llm", false, "Skip LLM output")
	cmd.Flags().BoolVar(&opts.noJSON, "no-json", false, "Skip JSON artifact output")
	cmd.Flags().BoolVar(&opts.publish, "publish", false, "Build hosted/served HTML assets without starting a local server")
	cmd.Flags().BoolVar(&opts.serve, "serve", false, "Serve the rendered output after building")
	cmd.Flags().IntVar(&opts.port, "port", 9090, "Port to use with --serve")
	cmd.Flags().BoolVar(&opts.debug, "debug", false, "Disable the progress bar and stream build logs live")

	cmd.SetOut(a.stdout)
	cmd.SetErr(a.stderr)
	return cmd
}

func (a *application) runRoot(cmd *cobra.Command, args []string, opts *rootOptions) error {
	fileConfig, err := loadPrintingPressConfig(opts.configPath, firstArg(args))
	if err != nil {
		return &cliError{
			message: "unable to load configuration",
			hint:    "Use --config /path/to/printing-press.yaml or place printing-press.yaml in the current or target directory.",
			detail:  err.Error(),
		}
	}
	applyConfigToRootOptions(cmd, opts, fileConfig)

	theme, err := parseTheme(opts.theme)
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
			hint:    "Try 'printing-press ./openapi.yaml', 'printing-press ./apis', or 'printing-press --serve ./openapi.yaml'.",
			detail:  fmt.Sprintf("received %d arguments", len(args)),
		}
	}

	inputArg, inputSet := resolveBuildInput(args, fileConfig)
	if !inputSet {
		a.printWelcome(opts, palette)
		return nil
	}

	return a.runBuild(inputArg, opts, palette, fileConfig)
}

func (a *application) runBuild(specArg string, opts *rootOptions, palette terminal.Palette, fileConfig *printingPressConfigFile) (err error) {
	if !isRemoteInput(specArg) {
		absPath, absErr := filepath.Abs(specArg)
		if absErr == nil {
			if info, statErr := os.Stat(absPath); statErr == nil && info.IsDir() {
				return a.runAggregateBuild(absPath, opts, palette, fileConfig)
			}
		}
	}
	return a.runSingleSpecBuild(specArg, opts, palette)
}

func (a *application) runSingleSpecBuild(specArg string, opts *rootOptions, palette terminal.Palette) (err error) {
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

	renderer := newActivityRenderer(renderMode, a.stderr, palette, buildStageCount(opts), loggerSession.logger)
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

	pp, err := printingpress.CreatePrintingPressFromBytes(source.specBytes, &printingpress.PrintingPressConfig{
		Title:     opts.title,
		BaseURL:   opts.baseURL,
		BasePath:  source.basePath,
		SpecPath:  source.specPath,
		OutputDir: outputDir,
		AssetMode: assetMode,
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
		htmlStats, err = runWithActivity(pp, renderer.renderActivity, pp.PrintHTML)
		if err != nil {
			return &cliError{message: "html render failed", detail: err.Error()}
		}
	}

	if !opts.noLLM {
		llmStats, err = runWithActivity(pp, renderer.renderActivity, pp.PrintLLM)
		if err != nil {
			return &cliError{message: "llm render failed", detail: err.Error()}
		}
	}

	if !opts.noJSON {
		if htmlStats == nil && llmStats == nil {
			site, err = runWithActivity(pp, renderer.renderActivity, pp.PressModel)
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
		renderer.updateManual("json", "writing json artifacts", "running", 0.2, 0, nil)
		if err := printingpress.PrintJSONArtifacts(site, ""); err != nil {
			renderer.updateManual("json", "json artifact write failed", "failed", 0, time.Since(jsonStart), err)
			return &cliError{message: "json artifact write failed", detail: err.Error()}
		}
		jsonDuration := time.Since(jsonStart)
		renderer.updateManual("json", "json artifacts complete", "completed", 1, jsonDuration, nil)
	}

	if site == nil {
		site, err = pp.PressModel()
		if err != nil {
			return &cliError{message: "model build failed", detail: err.Error()}
		}
	}

	fileCount, totalBytes, err := scanOutputDir(site.OutputDir)
	if err != nil {
		return &cliError{message: "unable to scan output directory", detail: err.Error()}
	}

	renderer.Close()
	a.printSummary(palette, site, htmlStats, llmStats, time.Since(buildStart), fileCount, totalBytes)

	if opts.serve {
		fmt.Fprintf(a.stdout, "serving http://127.0.0.1:%d from %s\n", opts.port, site.OutputDir)
		if err := a.serveFn(fmt.Sprintf(":%d", opts.port), site.OutputDir); err != nil {
			return &cliError{message: "unable to serve rendered output", detail: err.Error()}
		}
	}

	return nil
}

func (a *application) configureBuildLogger(palette terminal.Palette, mode activityRenderMode) *buildLoggerSession {
	buffered := mode == activityRenderModeProgress
	level := slog.LevelWarn
	if mode == activityRenderModeDebug {
		level = slog.LevelDebug
	}
	handler := terminal.NewPrettyHandler(&terminal.PrettyHandlerOptions{
		Level:      level,
		TimeFormat: terminal.TimeFormatTimeOnly,
		Writer:     a.stderr,
		Palette:    &palette,
		Buffer:     buffered,
	})
	previous := slog.Default()
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return &buildLoggerSession{
		previous: previous,
		logger:   logger,
		handler:  handler,
		buffered: buffered,
	}
}

func (s *buildLoggerSession) finish(runErr error) {
	if s == nil {
		return
	}
	if s.buffered && runErr != nil && s.handler != nil {
		_ = s.handler.Flush()
	}
	if s.previous != nil {
		slog.SetDefault(s.previous)
	}
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

	fmt.Fprintln(a.stdout, title.Render(">> Welcome! To render docs, try 'printing-press ./openapi.yaml'"))
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, muted.Render("Default outputs:"))
	fmt.Fprintln(a.stdout, "  > html site")
	fmt.Fprintln(a.stdout, "  > llm docs")
	fmt.Fprintln(a.stdout, "  > json bundle + artifacts")
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, muted.Render("Examples:"))
	fmt.Fprintln(a.stdout, "  "+accent.Render("printing-press ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render("printing-press ./apis"))
	fmt.Fprintln(a.stdout, "  "+accent.Render("printing-press --debug ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render("printing-press --publish --output ./api-docs ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render("printing-press --serve --output ./api-docs ./openapi.yaml"))
	fmt.Fprintln(a.stdout, "  "+accent.Render("printing-press https://example.com/openapi.yaml"))
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, title.Render("To see all the options, try 'printing-press --help'"))
	fmt.Fprintln(a.stdout)
}

func parseTheme(raw string) (terminal.ThemeName, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(terminal.ThemeDark):
		return terminal.ThemeDark, nil
	case "roger", string(terminal.ThemeLight):
		return terminal.ThemeLight, nil
	case string(terminal.ThemeTektronix):
		return terminal.ThemeTektronix, nil
	default:
		return "", fmt.Errorf("invalid theme %q: expected dark, roger, or tektronix", raw)
	}
}

func paletteForArgs(args []string) terminal.Palette {
	theme := terminal.ThemeDark
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--theme" && i+1 < len(args):
			if parsed, err := parseTheme(args[i+1]); err == nil {
				theme = parsed
			}
			i++
		case strings.HasPrefix(arg, "--theme="):
			if parsed, err := parseTheme(strings.TrimPrefix(arg, "--theme=")); err == nil {
				theme = parsed
			}
		}
	}
	return terminal.PaletteForTheme(theme)
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func resolveBuildInput(args []string, fileConfig *printingPressConfigFile) (string, bool) {
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

func runWithActivity[T any](pp *printingpress.PrintingPress, render func(*printingpress.ActivitySubscription), run func() (T, error)) (T, error) {
	sub := pp.ActivityStream()
	done := make(chan struct{})
	go func() {
		render(sub)
		close(done)
	}()
	result, err := run()
	if sub != nil {
		sub.Close()
	}
	select {
	case <-done:
	case <-time.After(activityRenderWaitTimeout):
		slog.Warn("activity renderer did not shut down before timeout", "timeout", roundDuration(activityRenderWaitTimeout).String())
	}
	return result, err
}

func (a *application) renderActivity(sub *printingpress.ActivitySubscription) {
	if sub == nil {
		return
	}
	printed := false
	lastLen := 0
	for update := range sub.Updates() {
		line := formatActivity(update)
		if line == "" {
			continue
		}
		padding := ""
		if diff := lastLen - len(line); diff > 0 {
			padding = strings.Repeat(" ", diff)
		}
		fmt.Fprintf(a.stderr, "\r%s%s", line, padding)
		printed = true
		lastLen = len(line)
	}
	if printed {
		fmt.Fprintln(a.stderr)
	}
}

func formatActivity(update printingpress.ActivityUpdate) string {
	label := strings.ToUpper(update.JobType)
	if label == "" {
		label = "WORK"
	}

	switch update.Status {
	case "completed":
		return fmt.Sprintf("[%s] completed in %s", label, roundDuration(update.Elapsed))
	case "failed":
		if update.Error != "" {
			return fmt.Sprintf("[%s] failed: %s", label, update.Error)
		}
		return fmt.Sprintf("[%s] failed", label)
	default:
		if update.TotalTasks > 0 {
			return fmt.Sprintf("[%s] %s (%d/%d %.0f%%)", label, update.CurrentTask, update.CompletedTasks, update.TotalTasks, update.PercentComplete)
		}
		return fmt.Sprintf("[%s] %s", label, update.CurrentTask)
	}
}

func formatStatusLine(label, message string) string {
	return fmt.Sprintf("[%s] %s", strings.ToUpper(label), message)
}

func roundDuration(d time.Duration) time.Duration {
	if d < time.Millisecond {
		return d
	}
	return d.Round(time.Millisecond)
}

func countModels(site *ppmodel.Site) int {
	total := 0
	for _, pages := range site.Models {
		total += len(pages)
	}
	return total
}

func scanOutputDir(root string) (int, int64, error) {
	var files int
	var totalBytes int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files++
		totalBytes += info.Size()
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("scan output directory: %w", err)
	}
	return files, totalBytes, nil
}

func humanBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func serveOutputDir(addr, dir string) error {
	return http.ListenAndServe(addr, newStaticServer(dir))
}

func newStaticServer(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyServeCacheHeaders(w.Header(), r.URL.Path)
		if !shouldGzipResponse(r) || !isCompressibleAsset(r.URL.Path) {
			fileServer.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")

		gzw := gzip.NewWriter(w)
		defer gzw.Close()

		fileServer.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, writer: gzw}, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.writer.Write(b)
}

func applyServeCacheHeaders(header http.Header, requestPath string) {
	switch cachePolicyForPath(requestPath) {
	case "revalidate":
		header.Set("Cache-Control", "no-cache")
	default:
		header.Set("Cache-Control", "no-store")
	}
}

func cachePolicyForPath(requestPath string) string {
	if requestPath == "" || requestPath == "/" {
		return "revalidate"
	}
	if strings.HasPrefix(requestPath, "/static/") ||
		strings.HasPrefix(requestPath, "/static/page-data/") ||
		strings.HasPrefix(requestPath, "/static/page-viz/") ||
		strings.HasPrefix(requestPath, "/static/printing-press-shared.") {
		return "revalidate"
	}
	if strings.EqualFold(filepath.Ext(requestPath), ".html") {
		return "revalidate"
	}
	return "no-store"
}

func shouldGzipResponse(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method != http.MethodGet {
		return false
	}
	if r.Header.Get("Range") != "" {
		return false
	}
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func isCompressibleAsset(requestPath string) bool {
	ext := strings.ToLower(filepath.Ext(requestPath))
	switch ext {
	case "", ".html", ".css", ".js", ".json", ".svg", ".txt", ".xml", ".map", ".md", ".markdown", ".yaml", ".yml":
		return true
	default:
		return false
	}
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
