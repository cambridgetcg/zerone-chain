package domains

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// datasetFiles maps domain names to their dataset file names.
var datasetFiles = map[string]string{
	"code":        "code_bench.json",
	"reasoning":   "reasoning_bench.json",
	"instruction": "instruct_bench.json",
}

// LoadAll loads benchmark cases from all domain datasets.
func LoadAll(datasetDir string) ([]BenchCase, error) {
	var all []BenchCase
	for domain := range datasetFiles {
		cases, err := LoadDomain(datasetDir, domain)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", domain, err)
		}
		all = append(all, cases...)
	}
	return all, nil
}

// LoadDomain loads benchmark cases for a specific domain.
func LoadDomain(datasetDir, domain string) ([]BenchCase, error) {
	filename, ok := datasetFiles[domain]
	if !ok {
		return nil, fmt.Errorf("unknown domain %q (valid: code, reasoning, instruction)", domain)
	}

	path := filepath.Join(datasetDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cases []BenchCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return cases, nil
}

// ListDomains returns the available domain names.
func ListDomains() []string {
	names := make([]string, 0, len(datasetFiles))
	for d := range datasetFiles {
		names = append(names, d)
	}
	return names
}
