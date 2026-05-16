package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	v3 "github.com/pb33f/doctor/model/high/v3"
	"github.com/pb33f/doctor/printingpress"
	ppmodel "github.com/pb33f/doctor/printingpress/model"
)

type serveArchiveDirs struct {
	root          string
	plain         string
	diagnostics   string
	llm           string
	diagnosticLLM string
}

func (d *serveArchiveDirs) cleanup() {
	if d != nil && d.root != "" {
		_ = os.RemoveAll(d.root)
	}
}

func renderServeArchiveDirs(source sourceInput, opts *rootOptions, lintResults []*v3.RuleFunctionResult, footer *ppmodel.FooterConfig) (*serveArchiveDirs, error) {
	if opts == nil || opts.noHTML {
		return nil, nil
	}

	root, err := os.MkdirTemp("", "printing-press-serve-archive-*")
	if err != nil {
		return nil, fmt.Errorf("create archive render directory: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(root)
		}
	}()

	plainDir := filepath.Join(root, "docs")
	if err := renderServeArchiveVariant(source, opts, plainDir, false, nil, footer, false); err != nil {
		return nil, err
	}

	diagnosticsDir := ""
	if len(lintResults) > 0 {
		diagnosticsDir = filepath.Join(root, "docs-diagnostics")
		if err := renderServeArchiveVariant(source, opts, diagnosticsDir, true, lintResults, footer, false); err != nil {
			return nil, err
		}
	}

	llmDir := ""
	diagnosticsLLMDir := ""
	if !opts.noLLM {
		llmDir = filepath.Join(root, "docs-llm")
		if err := renderServeArchiveVariant(source, opts, llmDir, false, nil, footer, true); err != nil {
			return nil, err
		}
		if len(lintResults) > 0 {
			diagnosticsLLMDir = filepath.Join(root, "docs-diagnostics-llm")
			if err := renderServeArchiveVariant(source, opts, diagnosticsLLMDir, true, lintResults, footer, true); err != nil {
				return nil, err
			}
		}
	}

	success = true
	return &serveArchiveDirs{
		root:          root,
		plain:         plainDir,
		diagnostics:   diagnosticsDir,
		llm:           llmDir,
		diagnosticLLM: diagnosticsLLMDir,
	}, nil
}

func renderServeArchiveVariant(source sourceInput, opts *rootOptions, outputDir string, developerMode bool, lintResults []*v3.RuleFunctionResult, footer *ppmodel.FooterConfig, includeLLM bool) error {
	pp, err := printingpress.CreatePrintingPressFromBytes(source.specBytes, &printingpress.PrintingPressConfig{
		Title:         opts.title,
		BasePath:      source.basePath,
		SpecPath:      source.specPath,
		OutputDir:     outputDir,
		AssetMode:     printingpress.HTMLAssetModePortable,
		DeveloperMode: developerMode,
		LintResults:   lintResults,
		Footer:        footer,
	})
	if err != nil {
		return fmt.Errorf("create archive printing press: %w", err)
	}
	if _, err := pp.PrintHTML(); err != nil {
		return fmt.Errorf("render archive html: %w", err)
	}
	if includeLLM {
		if _, err := pp.PrintLLM(); err != nil {
			return fmt.Errorf("render archive llm: %w", err)
		}
	}
	return nil
}
