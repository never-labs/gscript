package modpkg

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/never-labs/leia/internal/modfile"
)

type DownloadOptions struct {
	CacheDir      string
	GitHubBaseURL string
	Client        *http.Client
}

type DownloadReport struct {
	SchemaVersion int             `json:"schema_version"`
	OK            bool            `json:"ok"`
	Manifest      string          `json:"manifest,omitempty"`
	CacheDir      string          `json:"cache_dir,omitempty"`
	Modules       []DownloadEntry `json:"modules,omitempty"`
	Diagnostics   []Diagnostic    `json:"diagnostics,omitempty"`
}

type DownloadEntry struct {
	Path       string `json:"path"`
	Version    string `json:"version"`
	Repo       string `json:"repo"`
	Subdir     string `json:"subdir,omitempty"`
	URL        string `json:"url"`
	Zip        string `json:"zip"`
	ExtractDir string `json:"extract_dir"`
	Downloaded bool   `json:"downloaded"`
	Extracted  bool   `json:"extracted"`
}

func Download(path string, opts DownloadOptions) DownloadReport {
	abs, err := filepath.Abs(path)
	report := DownloadReport{SchemaVersion: 1}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Manifest = manifestPath
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
		return report
	}
	cacheDir, err := moduleCacheDir(opts.CacheDir)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9110", Message: err.Error()})
		return report
	}
	report.CacheDir = cacheDir
	report.Modules = append(report.Modules, downloadRequirements(abs, manifest, cacheDir, opts, &report.Diagnostics, map[string]bool{})...)
	if len(report.Diagnostics) == 0 {
		entries, diags := remoteSumEntries(abs, manifest, cacheDir)
		report.Diagnostics = append(report.Diagnostics, diags...)
		if len(diags) == 0 {
			sumPath := filepath.Join(abs, SumFileName)
			if err := updateSumFile(sumPath, entries); err != nil {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9109", Message: err.Error(), File: sumPath})
			}
		}
	}
	report.OK = len(report.Diagnostics) == 0
	return report
}

func downloadRequirements(root string, manifest modfile.File, cacheDir string, opts DownloadOptions, diags *[]Diagnostic, seen map[string]bool) []DownloadEntry {
	var modules []DownloadEntry
	for _, req := range manifest.Require {
		key := req.Path + "\x00" + req.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		if depRoot, kind, ok := dependencyRootWithCache(root, manifest, req, cacheDir); ok && kind == "replace" {
			depManifest, _, err := ReadFileWithPath(depRoot)
			if err != nil {
				continue
			}
			modules = append(modules, downloadRequirements(depRoot, depManifest, cacheDir, opts, diags, seen)...)
			continue
		}
		entry, err := downloadRequirement(req.Path, req.Version, cacheDir, opts)
		if err != nil {
			*diags = append(*diags, Diagnostic{Severity: "error", Code: "LEIA9111", Message: err.Error()})
			continue
		}
		modules = append(modules, entry)
		depRoot := cachedRequirementRoot(cacheDir, req.Path, req.Version)
		depManifest, _, err := ReadFileWithPath(depRoot)
		if err != nil {
			continue
		}
		modules = append(modules, downloadRequirements(depRoot, depManifest, cacheDir, opts, diags, seen)...)
	}
	return modules
}

func downloadRequirement(modulePath, version, cacheDir string, opts DownloadOptions) (DownloadEntry, error) {
	github, ok := parseGitHubModule(modulePath)
	if !ok {
		return DownloadEntry{}, fmt.Errorf("%s: only github.com module paths are supported by mod download today", modulePath)
	}
	if strings.TrimSpace(version) == "" {
		return DownloadEntry{}, fmt.Errorf("%s: require version is empty", modulePath)
	}
	baseURL := strings.TrimRight(opts.GitHubBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://github.com"
	}
	url := fmt.Sprintf("%s/%s/%s/archive/refs/tags/%s.zip", baseURL, github.Owner, github.RepoName, version)
	repoPath := filepath.FromSlash(github.Repo)
	zipPath := filepath.Join(cacheDir, "download", repoPath, "@v", version+".zip")
	extractDir := filepath.Join(cacheDir, "extract", filepath.FromSlash(github.Repo+"@"+version))
	entry := DownloadEntry{
		Path:       modulePath,
		Version:    version,
		Repo:       github.Repo,
		Subdir:     github.Subdir,
		URL:        url,
		Zip:        zipPath,
		ExtractDir: extractDir,
	}
	if _, err := os.Stat(extractDir); err == nil {
		entry.Extracted = false
		return entry, nil
	} else if err != nil && !os.IsNotExist(err) {
		return entry, err
	}
	if _, err := os.Stat(zipPath); err == nil {
		entry.Downloaded = false
	} else if err != nil && !os.IsNotExist(err) {
		return entry, err
	} else {
		if err := fetchFile(url, zipPath, opts.Client); err != nil {
			cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", github.Owner, github.RepoName)
			if cloneErr := cloneGitTag(cloneURL, version, extractDir); cloneErr != nil {
				return entry, fmt.Errorf("%v; git fallback failed: %v", err, cloneErr)
			}
			entry.Downloaded = true
			entry.Extracted = true
			return entry, nil
		}
		entry.Downloaded = true
	}
	if err := extractGitHubArchive(zipPath, extractDir); err != nil {
		return entry, err
	}
	entry.Extracted = true
	return entry, nil
}

func cloneGitTag(url, version, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	_ = os.RemoveAll(tmp)
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", version, url, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	_ = os.RemoveAll(filepath.Join(tmp, ".git"))
	if err := os.RemoveAll(dst); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

type githubModule struct {
	Owner    string
	RepoName string
	Repo     string
	Subdir   string
}

func parseGitHubModule(modulePath string) (githubModule, bool) {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 || parts[0] != "github.com" || parts[1] == "" || parts[2] == "" {
		return githubModule{}, false
	}
	out := githubModule{
		Owner:    parts[1],
		RepoName: parts[2],
		Repo:     strings.Join(parts[:3], "/"),
	}
	if len(parts) > 3 {
		out.Subdir = strings.Join(parts[3:], "/")
	}
	return out, true
}

func moduleCacheDir(override string) (string, error) {
	return ModuleCacheDir(override)
}

func ModuleCacheDir(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	if env := os.Getenv("LEIA_CACHE"); env != "" {
		return filepath.Abs(env)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "leia", "mod"), nil
}

func fetchFile(url, path string, client *http.Client) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, path)
}

func extractGitHubArchive(zipPath, dst string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries := 0
	for _, file := range reader.File {
		name := stripArchiveRoot(file.Name)
		if name == "" {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dstFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = src.Close()
			return err
		}
		_, copyErr := io.Copy(dstFile, src)
		closeSrcErr := src.Close()
		closeDstErr := dstFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeSrcErr != nil {
			return closeSrcErr
		}
		if closeDstErr != nil {
			return closeDstErr
		}
		entries++
	}
	if entries == 0 {
		return fmt.Errorf("archive has no files: %s", zipPath)
	}
	return nil
}

func stripArchiveRoot(name string) string {
	name = strings.TrimLeft(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == "" {
		return ""
	}
	if idx := strings.Index(name, "/"); idx >= 0 {
		return strings.TrimLeft(name[idx+1:], "/")
	}
	return ""
}
