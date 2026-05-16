package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const serveArchiveExportEndpoint = "/_printing-press/export"

type staticServerOptions struct {
	Dir                      string
	BaseURL                  string
	DisableExport            bool
	ArchiveDir               string
	DiagnosticsArchiveDir    string
	LLMArchiveDir            string
	DiagnosticsLLMArchiveDir string
}

func archiveExportURLForServe(opts *rootOptions) string {
	if opts == nil || !opts.serve || opts.disableExport {
		return ""
	}
	return archiveExportPathForBaseURL(opts.baseURL)
}

func archiveExportPathForBaseURL(baseURL string) string {
	mountPath := resolveServeMountPath(baseURL)
	if mountPath == "/" {
		return serveArchiveExportEndpoint
	}
	return strings.TrimSuffix(mountPath, "/") + serveArchiveExportEndpoint
}

func newStaticExportHandler(opts staticServerOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("Allow", "GET, HEAD, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		case http.MethodGet, http.MethodHead:
		default:
			w.Header().Set("Allow", "GET, HEAD, OPTIONS")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		format, filename, err := serveArchiveFormat(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		includeDiagnostics := serveArchiveIncludesDiagnostics(r)
		includeLLM := serveArchiveIncludesLLM(r)
		archiveDir := opts.ArchiveDir
		switch {
		case includeDiagnostics && includeLLM && opts.DiagnosticsLLMArchiveDir != "":
			archiveDir = opts.DiagnosticsLLMArchiveDir
		case includeDiagnostics && opts.DiagnosticsArchiveDir != "":
			archiveDir = opts.DiagnosticsArchiveDir
		case includeLLM && opts.LLMArchiveDir != "":
			archiveDir = opts.LLMArchiveDir
		}
		if archiveDir == "" {
			archiveDir = opts.Dir
		}
		if err := serveStaticArchive(w, r, archiveDir, format, filename, includeDiagnostics); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func serveArchiveFormat(r *http.Request) (format string, filename string, err error) {
	value := ""
	if r != nil && r.URL != nil {
		value = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	}
	switch value {
	case "", "zip":
		return "zip", "printing-press-docs.zip", nil
	case "tar.gz", "tgz":
		return "tar.gz", "printing-press-docs.tar.gz", nil
	default:
		return "", "", fmt.Errorf("unsupported archive format %q", value)
	}
}

func serveArchiveIncludesDiagnostics(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("diagnostics")))
	return value == "true"
}

func serveArchiveIncludesLLM(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("llm")))
	return value == "true"
}

func serveStaticArchive(w http.ResponseWriter, r *http.Request, dir, format, filename string, includeDiagnostics bool) error {
	suffix := ".zip"
	if format == "tar.gz" {
		suffix = ".tar.gz"
	}
	tmp, err := os.CreateTemp("", "printing-press-docs-*"+suffix)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	switch format {
	case "zip":
		err = writeZipArchive(tmp, dir, includeDiagnostics)
	case "tar.gz":
		err = writeTarGzArchive(tmp, dir, includeDiagnostics)
	default:
		err = fmt.Errorf("unsupported archive format %q", format)
	}
	if err != nil {
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind archive: %w", err)
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	switch format {
	case "zip":
		w.Header().Set("Content-Type", "application/zip")
	case "tar.gz":
		w.Header().Set("Content-Type", "application/gzip")
	}
	http.ServeContent(w, r, filename, time.Now(), tmp)
	return nil
}

func writeZipArchive(out io.Writer, root string, includeDiagnostics bool) error {
	archive := zip.NewWriter(out)
	defer archive.Close()

	return filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skipServedArchiveFile(rel, includeDiagnostics) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func writeTarGzArchive(out io.Writer, root string, includeDiagnostics bool) error {
	gzipWriter := gzip.NewWriter(out)
	defer gzipWriter.Close()

	archive := tar.NewWriter(gzipWriter)
	defer archive.Close()

	return filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skipServedArchiveFile(rel, includeDiagnostics) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if err := archive.WriteHeader(header); err != nil {
			return err
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(archive, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func skipServedArchiveFile(rel string, includeDiagnostics bool) bool {
	if includeDiagnostics {
		return false
	}
	clean := strings.TrimPrefix(path.Clean("/"+rel), "/")
	base := path.Base(clean)
	return base == "diagnostics.html" ||
		base == "diagnostics-orphans.json" ||
		strings.HasPrefix(clean, "data/pages/diagnostics")
}
