package rollback

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// Manager handles model rollback operations.
type Manager struct {
	modelsDir   string
	maxVersions int
}

// NewManager creates a rollback manager.
func NewManager(modelsDir string, maxVersions int) *Manager {
	if maxVersions == 0 {
		maxVersions = 5
	}
	return &Manager{
		modelsDir:   modelsDir,
		maxVersions: maxVersions,
	}
}

// ListVersions returns available adapter versions, sorted newest first.
func (m *Manager) ListVersions() ([]string, error) {
	dir := filepath.Join(m.modelsDir, "adapters")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	// Sort reverse (newest version name first, assuming semantic versioning)
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	return versions, nil
}

// Rollback switches the active adapter to a previous version.
func (m *Manager) Rollback(targetVersion string) error {
	adapterPath := filepath.Join(m.modelsDir, "adapters", targetVersion)
	if _, err := os.Stat(adapterPath); os.IsNotExist(err) {
		return fmt.Errorf("version not found: %s", targetVersion)
	}

	activePath := filepath.Join(m.modelsDir, "active")
	_ = os.Remove(activePath)
	if err := os.Symlink(filepath.Join("adapters", targetVersion), activePath); err != nil {
		return fmt.Errorf("rollback symlink: %w", err)
	}

	log.Printf("rollback: switched active to %s", targetVersion)
	return nil
}

// Cleanup removes old adapter versions beyond maxVersions.
func (m *Manager) Cleanup() ([]string, error) {
	versions, err := m.ListVersions()
	if err != nil {
		return nil, err
	}

	if len(versions) <= m.maxVersions {
		return nil, nil
	}

	var removed []string
	for _, v := range versions[m.maxVersions:] {
		path := filepath.Join(m.modelsDir, "adapters", v)
		if err := os.RemoveAll(path); err != nil {
			log.Printf("cleanup: failed to remove %s: %v", v, err)
			continue
		}
		removed = append(removed, v)
	}

	return removed, nil
}
