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
	SchemaVersion   int           `json:"schema_version"`
	OK              bool          `json:"ok"`
	Manifest        string        `json:"manifest,omitempty"`
	VendorDir       string        `json:"vendor_dir,omitempty"`
	ModuleCount     int           `json:"module_count"`
	DiagnosticCount int           `json:"diagnostic_count"`
	Modules         []VendorEntry `json:"modules"`
	Diagnostics     []Diagnostic  `json:"diagnostics"`
}

type VendorEntry struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Target  string `json:"target"`
}

func Vendor(path string, opts VendorOptions) (report VendorReport) {
	abs, err := filepath.Abs(path)
	report = VendorReport{SchemaVersion: 1}
	defer setVendorReportCounts(&report)
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
	vendorDir := opts.VendorDir
	if vendorDir == "" {
		vendorDir = filepath.Join(abs, "vendor")
	} else if !filepath.IsAbs(vendorDir) {
		vendorDir = filepath.Join(abs, vendorDir)
	}
	report.VendorDir = vendorDir
	if opts.Clear {
		if err := os.RemoveAll(vendorDir); err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9112", Message: err.Error(), File: vendorDir})
			return report
		}
	}
	cacheDir, err := ModuleCacheDir(opts.CacheDir)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9110", Message: err.Error()})
		return report
	}
	for _, dep := range dependencyClosure(abs, manifest, cacheDir) {
		if dep.Kind == "replace" {
			continue
		}
		req := dep.Require
		entry, err := vendorRequirement(cacheDir, vendorDir, req.Path, req.Version)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9113", Message: err.Error()})
			continue
		}
		report.Modules = append(report.Modules, entry)
	}
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

func setVendorReportCounts(report *VendorReport) {
	report.ModuleCount = len(report.Modules)
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Modules == nil {
		report.Modules = []VendorEntry{}
	}
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func vendorRequirement(cacheDir, vendorDir, modulePath, version string) (VendorEntry, error) {
	if version == "" {
		return VendorEntry{}, fmt.Errorf("%s: require version is empty", modulePath)
	}
	source := cachedRequirementRoot(cacheDir, modulePath, version)
	if _, err := os.Stat(source); err != nil {
		return VendorEntry{}, fmt.Errorf("%s@%s is not downloaded; run leia mod download", modulePath, version)
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
