package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	gatekeeperLibraryAPI = "https://api.github.com/repos/open-policy-agent/gatekeeper-library/contents"
	rawContentBase       = "https://raw.githubusercontent.com/open-policy-agent/gatekeeper-library/master"
)

// Scraper fetches constraints from the Gatekeeper library
type Scraper struct {
	OutputDir  string
	Categories string
	Format     string
}

// Run executes the scraper
func (s *Scraper) Run() error {
	// Create output directory
	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	library := ConstraintLibrary{
		FetchedAt: time.Now(),
	}

	// Fetch constraints for each category
	for _, category := range strings.Split(s.Categories, ",") {
		category = strings.TrimSpace(category)
		fmt.Printf("Fetching constraints from category: %s\n", category)

		constraints, err := s.fetchCategoryConstraints(category)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching category %s: %v\n", category, err)
			continue
		}
		library.Constraints = append(library.Constraints, constraints...)
	}

	// Save the library
	outputFile := filepath.Join(s.OutputDir, "constraints."+s.Format)
	if err := s.saveLibrary(library, outputFile); err != nil {
		return fmt.Errorf("saving library: %w", err)
	}
	fmt.Printf("Saved %d constraints to %s\n", len(library.Constraints), outputFile)

	return nil
}

func (s *Scraper) fetchCategoryConstraints(category string) ([]Constraint, error) {
	url := fmt.Sprintf("%s/library/%s", gatekeeperLibraryAPI, category)

	contents, err := fetchGitHubContents(url)
	if err != nil {
		return nil, fmt.Errorf("fetching category contents: %w", err)
	}

	var constraints []Constraint
	for _, item := range contents {
		if item.Type != "dir" || item.Name == "kustomization.yaml" {
			continue
		}

		fmt.Printf("  Fetching constraint: %s\n", item.Name)
		constraint, err := s.fetchConstraint(category, item.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: error fetching %s: %v\n", item.Name, err)
			continue
		}
		constraints = append(constraints, *constraint)
	}

	return constraints, nil
}

func (s *Scraper) fetchConstraint(category, name string) (*Constraint, error) {
	constraint := &Constraint{
		Name:     name,
		Category: category,
	}

	// Fetch template
	templateURL := fmt.Sprintf("%s/library/%s/%s/template.yaml", rawContentBase, category, name)
	templateContent, err := fetchRawContent(templateURL)
	if err != nil {
		return nil, fmt.Errorf("fetching template: %w", err)
	}
	constraint.Template = templateContent

	// Extract description from template
	constraint.Description = extractDescription(templateContent)

	// Fetch samples
	samplesURL := fmt.Sprintf("%s/library/%s/%s/samples", gatekeeperLibraryAPI, category, name)
	sampleDirs, err := fetchGitHubContents(samplesURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "    Warning: no samples found for %s\n", name)
		return constraint, nil
	}

	for _, sampleDir := range sampleDirs {
		if sampleDir.Type != "dir" {
			continue
		}

		sample, err := s.fetchSample(category, name, sampleDir.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    Warning: error fetching sample %s: %v\n", sampleDir.Name, err)
			continue
		}
		constraint.Samples = append(constraint.Samples, *sample)
	}

	return constraint, nil
}

func (s *Scraper) fetchSample(category, constraintName, sampleName string) (*ConstraintSample, error) {
	sample := &ConstraintSample{
		Name: sampleName,
	}

	basePath := fmt.Sprintf("%s/library/%s/%s/samples/%s", rawContentBase, category, constraintName, sampleName)

	// Fetch constraint
	constraintContent, err := fetchRawContent(basePath + "/constraint.yaml")
	if err == nil {
		sample.Constraint = constraintContent
	}

	// Fetch all files in the sample directory to find examples
	filesURL := fmt.Sprintf("%s/library/%s/%s/samples/%s", gatekeeperLibraryAPI, category, constraintName, sampleName)
	files, err := fetchGitHubContents(filesURL)
	if err != nil {
		return sample, nil
	}

	for _, file := range files {
		if file.Type != "file" || !strings.HasSuffix(file.Name, ".yaml") {
			continue
		}

		content, err := fetchRawContent(file.DownloadURL)
		if err != nil {
			continue
		}

		// Categorize the example
		nameLower := strings.ToLower(file.Name)
		if strings.Contains(nameLower, "allowed") && !strings.Contains(nameLower, "disallowed") {
			sample.AllowedExamples = append(sample.AllowedExamples, content)
		} else if strings.Contains(nameLower, "disallowed") || strings.Contains(nameLower, "violation") {
			sample.DisallowedExamples = append(sample.DisallowedExamples, content)
		}
	}

	return sample, nil
}

func (s *Scraper) saveLibrary(library ConstraintLibrary, path string) error {
	var data []byte
	var err error

	if s.Format == "json" {
		data, err = json.MarshalIndent(library, "", "  ")
	} else {
		data, err = yaml.Marshal(library)
	}

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func fetchGitHubContents(url string) ([]GitHubContent, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	var contents []GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	return contents, nil
}

func fetchRawContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func extractDescription(template string) string {
	// Parse YAML and extract description annotation
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(template), &data); err != nil {
		return ""
	}

	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}

	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		return ""
	}

	desc, ok := annotations["description"].(string)
	if !ok {
		return ""
	}

	return desc
}
