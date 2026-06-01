package modpkg

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type VendorOptions struct {
	CacheDir  string
	VendorDir string
	Clear     bool
}

type VendorReport struct {
	SchemaVersion int           `json:"schema_version"`
	OK            bool          `json:"ok"`
	Manifest      string        `json:"manifest,omitempty"`
	VendorDir     string        `json:"vendor_dir,omitempty"`
	Modules       []VendorEntry `json:"modules,omitempty"`
	Diagnostics   []Diagnostic  `json:"diagnostics,omitempty"`
}

type VendorEntry struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Target  string `json:"target"`
}

func Vendor(path string, opts VendorOptions) VendorReport {
	abs, err := filepath.Abs(path)
	report := VendorReport{SchemaVersion: 1}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Manifest = manifestPath
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath})
		return report
	}
	vendorDir := opts.VendorDir
	if vendorDir == "" {
		vendorDir = filepath.Join(abs, "vendor")
	} else if !filepath.IsAbs(vendorDir) {
		vendorDir = filepath.Join(abs, vendorDir)
	}
	report.VendorDir = vendorDir
	if opts.Clear {
		if err := os.RemoveAll(vendorDir); err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9112", Message: err.Error(), File: vendorDir})
			return report
		}
	}
	cacheDir, err := ModuleCacheDir(opts.CacheDir)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9110", Message: err.Error()})
		return report
	}
	for _, req := range manifest.Require {
		entry, err := vendorRequirement(cacheDir, vendorDir, req.Path, req.Version)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9113", Message: err.Error()})
			continue
		}
		report.Modules = append(report.Modules, entry)
	}
	report.OK = len(report.Diagnostics) == 0
	return report
}

func vendorRequirement(cacheDir, vendorDir, modulePath, version string) (VendorEntry, error) {
	if version == "" {
		return VendorEntry{}, fmt.Errorf("%s: require version is empty", modulePath)
	}
	source := filepath.Join(cacheDir, "extract", filepath.FromSlash(modulePath+"@"+version))
	if _, err := os.Stat(source); err != nil {
		return VendorEntry{}, fmt.Errorf("%s@%s is not downloaded; run gscript mod download", modulePath, version)
	}
	target := filepath.Join(vendorDir, filepath.FromSlash(modulePath+"@"+version))
	if err := os.RemoveAll(target); err != nil {
		return VendorEntry{}, err
	}
	if err := copyDir(source, target); err != nil {
		return VendorEntry{}, err
	}
	return VendorEntry{Path: modulePath, Version: version, Source: source, Target: target}, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
