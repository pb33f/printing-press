package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type printingPressConfigFile struct {
	configDir   string
	Title       string                      `mapstructure:"title"`
	Description string                      `mapstructure:"description"`
	Output      string                      `mapstructure:"output"`
	BaseURL     string                      `mapstructure:"baseURL"`
	BasePath    string                      `mapstructure:"basePath"`
	Theme       string                      `mapstructure:"theme"`
	NoLogo      bool                        `mapstructure:"noLogo"`
	NoHTML      bool                        `mapstructure:"noHTML"`
	NoLLM       bool                        `mapstructure:"noLLM"`
	NoJSON      bool                        `mapstructure:"noJSON"`
	Publish     bool                        `mapstructure:"publish"`
	Serve       bool                        `mapstructure:"serve"`
	Debug       bool                        `mapstructure:"debug"`
	Port        int                         `mapstructure:"port"`
	Scan        printingPressScanConfig     `mapstructure:"scan"`
	Grouping    printingPressGroupingConfig `mapstructure:"grouping"`
	Build       printingPressBuildConfig    `mapstructure:"build"`
	State       printingPressStateConfig    `mapstructure:"state"`
}

type printingPressScanConfig struct {
	Root        string   `mapstructure:"root"`
	Include     []string `mapstructure:"include"`
	IgnoreRules []string `mapstructure:"ignoreRules"`
}

type printingPressGroupingConfig struct {
	NoiseSegments        []string                  `mapstructure:"noiseSegments"`
	ServiceOverrides     []printingPressPathConfig `mapstructure:"serviceOverrides"`
	DisplayNameOverrides []printingPressPathConfig `mapstructure:"displayNameOverrides"`
	VersionOverrides     []printingPressPathConfig `mapstructure:"versionOverrides"`
}

type printingPressPathConfig struct {
	Pattern string `mapstructure:"pattern"`
	Value   string `mapstructure:"value"`
}

type printingPressBuildConfig struct {
	Mode                    string `mapstructure:"mode"`
	MaxPools                int    `mapstructure:"maxPools"`
	WorkersPerPool          int    `mapstructure:"workersPerPool"`
	DisableSkippedRendering bool   `mapstructure:"disableSkippedRendering"`
}

type printingPressStateConfig struct {
	Namespace string                       `mapstructure:"namespace"`
	SQLite    printingPressSQLiteStateFile `mapstructure:"sqlite"`
}

type printingPressSQLiteStateFile struct {
	Path string `mapstructure:"path"`
}

func loadPrintingPressConfig(configPath, inputArg string) (*printingPressConfigFile, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	if strings.TrimSpace(configPath) != "" {
		v.SetConfigFile(configPath)
	} else {
		configFile, ok := discoverConfigFile(autoConfigSearchPaths(inputArg))
		if !ok {
			return nil, nil
		}
		v.SetConfigFile(configFile)
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, err
	}

	var fileConfig printingPressConfigFile
	if err := v.Unmarshal(&fileConfig); err != nil {
		return nil, err
	}
	fileConfig.configDir = filepath.Dir(v.ConfigFileUsed())
	fileConfig.resolveRelativePaths()
	return &fileConfig, nil
}

func autoConfigSearchPaths(inputArg string) []string {
	paths := make([]string, 0, 2)
	if strings.TrimSpace(inputArg) == "" || isRemoteInput(inputArg) {
		return []string{"."}
	}
	absPath, err := filepath.Abs(inputArg)
	if err != nil {
		return []string{"."}
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return []string{"."}
	}
	if info.IsDir() {
		paths = append(paths, absPath)
	} else {
		paths = append(paths, filepath.Dir(absPath))
	}
	paths = append(paths, ".")
	return dedupeStrings(paths)
}

func (c *printingPressConfigFile) resolveRelativePaths() {
	if c == nil || c.configDir == "" {
		return
	}
	c.Output = resolveConfigRelativePath(c.configDir, c.Output)
	c.BasePath = resolveConfigRelativePath(c.configDir, c.BasePath)
	c.Scan.Root = resolveConfigRelativePath(c.configDir, c.Scan.Root)
	c.State.SQLite.Path = resolveConfigRelativePath(c.configDir, c.State.SQLite.Path)
}

func resolveConfigRelativePath(baseDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(baseDir, raw)
}

func applyConfigToRootOptions(cmd *cobra.Command, opts *rootOptions, fileConfig *printingPressConfigFile) {
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
	applyBoolFlag(cmd, "disable-skipped-rendering", &opts.disableSkippedRendering, fileConfig.Build.DisableSkippedRendering)
	applyBoolFlag(cmd, "no-logo", &opts.noLogo, fileConfig.NoLogo)
	applyBoolFlag(cmd, "no-html", &opts.noHTML, fileConfig.NoHTML)
	applyBoolFlag(cmd, "no-llm", &opts.noLLM, fileConfig.NoLLM)
	applyBoolFlag(cmd, "no-json", &opts.noJSON, fileConfig.NoJSON)
	applyBoolFlag(cmd, "publish", &opts.publish, fileConfig.Publish)
	applyBoolFlag(cmd, "serve", &opts.serve, fileConfig.Serve)
	applyBoolFlag(cmd, "debug", &opts.debug, fileConfig.Debug)
	applyIntFlag(cmd, "port", &opts.port, fileConfig.Port)

	if !cmd.Flags().Changed("title") {
		opts.description = strings.TrimSpace(fileConfig.Description)
	}
}

func discoverConfigFile(searchPaths []string) (string, bool) {
	candidates := []string{"printing-press.yaml", "printing-press.yml"}
	for _, searchPath := range searchPaths {
		if strings.TrimSpace(searchPath) == "" {
			continue
		}
		for _, candidate := range candidates {
			fullPath := filepath.Join(searchPath, candidate)
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				return fullPath, true
			}
		}
	}
	return "", false
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

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}
