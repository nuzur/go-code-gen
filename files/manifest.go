package files

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ManifestFileName is the name of the generation manifest written at the root of
// a generated project. It lists every file the generator produced, so tooling
// can compare runs and remove files that are no longer generated without
// touching files the user added.
const ManifestFileName = ".nuzur-codegen-manifest.json"

// ManifestVersion is the schema version of the manifest format.
const ManifestVersion = 1

// Manifest records the set of files produced by a generation run.
type Manifest struct {
	Version     int      `json:"version"`
	Generator   string   `json:"generator"`
	GeneratedAt string   `json:"generated_at"`
	// Files are project-root-relative, forward-slash paths, sorted.
	Files []string `json:"files"`
}

// WriteManifest scans rootDir for generated files (those carrying a generated
// marker) and writes the manifest to rootDir/ManifestFileName. It returns the
// manifest it wrote.
func WriteManifest(rootDir string) (Manifest, error) {
	generated, err := scanGeneratedFiles(rootDir)
	if err != nil {
		return Manifest{}, err
	}

	m := Manifest{
		Version:     ManifestVersion,
		Generator:   "nuzur go-code-gen",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Files:       generated,
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(rootDir, ManifestFileName), data, 0644); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ReadManifest reads the manifest from rootDir. If none exists it returns an
// empty manifest and no error, so callers can treat a first run uniformly.
func ReadManifest(rootDir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, ManifestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, nil
		}
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// StalePaths returns the files present in m (a previous manifest) that are no
// longer present in next (the current manifest) — i.e. files that were
// generated before but are not produced by the current configuration. These are
// the deletion candidates for a stale-file cleanup.
func (m Manifest) StalePaths(next Manifest) []string {
	current := make(map[string]struct{}, len(next.Files))
	for _, f := range next.Files {
		current[f] = struct{}{}
	}
	var stale []string
	for _, f := range m.Files {
		if _, ok := current[f]; !ok {
			stale = append(stale, f)
		}
	}
	sort.Strings(stale)
	return stale
}

// scanGeneratedFiles walks rootDir and returns the relative paths of every
// regular file that carries a generated marker, sorted.
func scanGeneratedFiles(rootDir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(rootDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ManifestFileName {
			return nil
		}
		if IsGenerated(p) {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
