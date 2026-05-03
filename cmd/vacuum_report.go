package cmd

import (
	"fmt"
	"io"
	"strings"

	vacuumModel "github.com/daveshanley/vacuum/model"
	"github.com/daveshanley/vacuum/rulesets"
	vacuumReport "github.com/daveshanley/vacuum/vacuum-report"
	docv3 "github.com/pb33f/doctor/model/high/v3"
	"go.yaml.in/yaml/v4"
)

func resolveDeveloperDiagnostics(opts *rootOptions, input io.Reader) (bool, []*docv3.RuleFunctionResult, error) {
	if opts == nil {
		return false, nil, nil
	}

	reportPath := strings.TrimSpace(opts.vacuumReport)
	if reportPath != "" && opts.vacuumReportStdin {
		return false, nil, fmt.Errorf("use either --stdin or --vacuum-report, not both")
	}
	if opts.vacuumReportStdin {
		results, err := loadDoctorLintResultsFromReader(input)
		return true, results, err
	}
	if reportPath == "" {
		return false, nil, nil
	}

	results, err := loadDoctorLintResults(reportPath)
	return true, results, err
}

func hasDeveloperDiagnosticsInput(opts *rootOptions) bool {
	return opts != nil && (opts.vacuumReportStdin || strings.TrimSpace(opts.vacuumReport) != "")
}

func loadDoctorLintResults(reportPath string) ([]*docv3.RuleFunctionResult, error) {
	reportPath = strings.TrimSpace(reportPath)
	if reportPath == "" {
		return nil, nil
	}

	vr, _, err := vacuumReport.BuildVacuumReportFromFile(reportPath)
	if err != nil {
		return nil, err
	}
	if vr == nil || vr.ResultSet == nil {
		return nil, fmt.Errorf("file is not a vacuum report: %s", reportPath)
	}

	return convertVacuumReportLintResults(vr), nil
}

func loadDoctorLintResultsFromReader(input io.Reader) ([]*docv3.RuleFunctionResult, error) {
	if input == nil {
		return nil, fmt.Errorf("stdin is not available")
	}

	data, err := io.ReadAll(input)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("stdin did not contain a vacuum report")
	}

	vr, err := vacuumReport.CheckFileForVacuumReport(data)
	if err != nil {
		return nil, err
	}
	if vr == nil || vr.ResultSet == nil {
		return nil, fmt.Errorf("stdin is not a vacuum report")
	}

	hydrateVacuumReport(vr)
	return convertVacuumReportLintResults(vr), nil
}

func convertVacuumReportLintResults(vr *vacuumReport.VacuumReport) []*docv3.RuleFunctionResult {
	if vr == nil || vr.ResultSet == nil {
		return nil
	}
	results := make([]*docv3.RuleFunctionResult, 0, len(vr.ResultSet.Results))
	for _, result := range vr.ResultSet.Results {
		if result == nil {
			continue
		}
		results = append(results, docv3.ConvertRuleResult(result))
	}
	return results
}

func hydrateVacuumReport(vr *vacuumReport.VacuumReport) {
	if vr == nil || vr.ResultSet == nil {
		return
	}
	rules := vacuumReportRules(vr)
	for _, result := range vr.ResultSet.Results {
		hydrateVacuumReportResult(result, rules)
	}
}

func vacuumReportRules(vr *vacuumReport.VacuumReport) map[string]*vacuumModel.Rule {
	rules := make(map[string]*vacuumModel.Rule)
	defaultRuleSets := rulesets.BuildDefaultRuleSets()
	defaultRules := defaultRuleSets.GenerateOpenAPIDefaultRuleSet()
	for id, rule := range defaultRules.Rules {
		rules[id] = rule
	}
	if vr != nil {
		for id, rule := range vr.Rules {
			rules[id] = rule
		}
	}
	return rules
}

func hydrateVacuumReportResult(result *vacuumModel.RuleFunctionResult, rules map[string]*vacuumModel.Rule) {
	if result == nil {
		return
	}
	result.StartNode = &yaml.Node{
		Line:   result.Range.Start.Line,
		Column: result.Range.Start.Char,
	}
	result.EndNode = &yaml.Node{
		Line:   result.Range.End.Line,
		Column: result.Range.End.Char,
	}
	if result.RuleId != "" {
		result.Rule = rules[result.RuleId]
	}
}
