package cmd

import (
	"runtime/debug"
	"strings"
	"time"
)

const buildDateFormat = "Mon, 02 Jan 2006 15:04:05 MST"

func resolveBuildInfo(version, commit, date string) BuildInfo {
	info := BuildInfo{
		Version: "dev",
		Commit:  "unknown",
		Date:    "unknown",
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = buildInfo.GoVersion
		if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			info.Version = buildInfo.Main.Version
		}
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				info.Commit = shortRevision(setting.Value)
			case "vcs.time":
				info.Date = formatBuildDate(setting.Value)
			case "vcs.modified":
				info.Modified = setting.Value == "true"
			}
		}
	}

	if trimmed := strings.TrimSpace(version); trimmed != "" {
		info.Version = trimmed
	}
	if trimmed := strings.TrimSpace(commit); trimmed != "" {
		info.Commit = shortRevision(trimmed)
	}
	if trimmed := strings.TrimSpace(date); trimmed != "" {
		info.Date = formatBuildDate(trimmed)
	}

	if info.Modified && info.Commit != "" && info.Commit != "unknown" && !strings.Contains(info.Commit, "+dirty") {
		info.Commit += "+dirty"
	}

	return info
}

func shortRevision(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "unknown"
	}
	if len(trimmed) > 7 {
		return trimmed[:7]
	}
	return trimmed
}

func formatBuildDate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "unknown"
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.Format(buildDateFormat)
	}
	return trimmed
}
