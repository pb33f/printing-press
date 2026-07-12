package cmd

import (
	v3 "github.com/pb33f/doctor/model/high/v3"
	ppmodel "github.com/pb33f/doctor/printingpress/model"
	ppserve "github.com/pb33f/doctor/printingpress/serve"
)

type serveArchiveDirs = ppserve.ArchiveVariantDirs

func renderServeArchiveDirs(source sourceInput, opts *rootOptions, lintResults []*v3.RuleFunctionResult, footer *ppmodel.FooterConfig) (*serveArchiveDirs, error) {
	if opts == nil {
		return nil, nil
	}
	return ppserve.RenderArchiveVariants(ppserve.ArchiveRenderOptions{
		Title:                              opts.title,
		BasePath:                           source.basePath,
		SpecPath:                           source.specPath,
		SpecBytes:                          source.specBytes,
		IncludeSpec:                        opts.includeSpec,
		LintResults:                        lintResults,
		Footer:                             footer,
		MaxPatternRepeatBudget:             opts.maxPatternRepeatBudget,
		MaxGeneratedStringBytes:            opts.maxGeneratedStringBytes,
		MaxGeneratedMockBytes:              opts.maxGeneratedMockBytes,
		LLMAggregateSpecSizeThresholdBytes: opts.llmAggregateSpecSizeThresholdBytes,
		LLMMaxAggregateFileBytes:           opts.llmMaxAggregateFileBytes,
		LLMGenerateMonoliths:               opts.llmGenerateMonoliths,
		IncludeLLM:                         !opts.noLLM,
		NoHTML:                             opts.noHTML,
	})
}
