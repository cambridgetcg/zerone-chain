package deployer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Config for model deployment.
type Config struct {
	ModelsDir    string // /models/ base directory
	InferenceURL string // vLLM server URL
	CanaryPct    int    // Initial canary traffic percentage (0-100)
	DrainTimeout time.Duration
}

// Deployer handles model hot-swap and deployment.
type Deployer struct {
	cfg Config
}

// New creates a new Deployer.
func New(cfg Config) *Deployer {
	if cfg.DrainTimeout == 0 {
		cfg.DrainTimeout = 30 * time.Second
	}
	return &Deployer{cfg: cfg}
}

// Deploy performs a hot-swap deployment of a new adapter.
func (d *Deployer) Deploy(ctx context.Context, adapterName string, canaryPct int) error {
	adapterPath := filepath.Join(d.cfg.ModelsDir, "adapters", adapterName)
	if _, err := os.Stat(adapterPath); os.IsNotExist(err) {
		return fmt.Errorf("adapter not found: %s", adapterPath)
	}

	log.Printf("deploy: starting hot-swap to %s (canary=%d%%)", adapterName, canaryPct)

	// Step 1: Update active symlink
	activePath := filepath.Join(d.cfg.ModelsDir, "active")
	_ = os.Remove(activePath)
	if err := os.Symlink(filepath.Join("adapters", adapterName), activePath); err != nil {
		return fmt.Errorf("update active symlink: %w", err)
	}

	// Step 2: Signal vLLM to reload (in production, use vLLM's adapter management API)
	if err := d.reloadInference(ctx); err != nil {
		log.Printf("deploy: reload signal warning: %v", err)
	}

	// Step 3: Wait for health
	if err := d.waitHealthy(ctx); err != nil {
		return fmt.Errorf("inference unhealthy after deploy: %w", err)
	}

	log.Printf("deploy: %s deployed successfully", adapterName)
	return nil
}

// Promote sets an adapter as the sole serving model (100% traffic).
func (d *Deployer) Promote(adapterName string) error {
	log.Printf("deploy: promoting %s to 100%% traffic", adapterName)
	// In production: update gateway routing config to route all traffic to this adapter
	return nil
}

// ListAvailable returns all adapter directories.
func (d *Deployer) ListAvailable() ([]string, error) {
	dir := filepath.Join(d.cfg.ModelsDir, "adapters")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ActiveAdapter returns the currently active adapter name.
func (d *Deployer) ActiveAdapter() (string, error) {
	activePath := filepath.Join(d.cfg.ModelsDir, "active")
	target, err := os.Readlink(activePath)
	if err != nil {
		return "", fmt.Errorf("no active adapter: %w", err)
	}
	return filepath.Base(target), nil
}

func (d *Deployer) reloadInference(ctx context.Context) error {
	// vLLM doesn't have a reload endpoint by default;
	// in production this would use the LoRA adapter management API
	// or trigger a container restart with zero-downtime.
	return nil
}

func (d *Deployer) waitHealthy(ctx context.Context) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(2 * time.Minute)

	for time.Now().Before(deadline) {
		resp, err := client.Get(d.cfg.InferenceURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for healthy inference server")
}
