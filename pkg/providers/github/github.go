package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/galbakal/k8s-ingress-meta-sync/pkg/model"
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/providers"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	providers.Register("github", func() providers.Provider {
		return &GitHubProvider{}
	})
}

// GitHubIPMetadata represents the structure of GitHub's IP metadata response
type GitHubIPMetadata struct {
	Hooks      []string `json:"hooks"`
	Web        []string `json:"web"`
	API        []string `json:"api"`
	Git        []string `json:"git"`
	Pages      []string `json:"pages"`
	Importer   []string `json:"importer"`
	Actions    []string `json:"actions"`
	Dependabot []string `json:"dependabot"`
}

// GitHubProvider implements the Provider interface for GitHub IP ranges
type GitHubProvider struct {
	name        string
	enterprise  bool
	apiToken    string
	cacheTTL    time.Duration
	lastFetch   time.Time
	cachedData  *model.IPRangeSet
	cacheMutex  sync.RWMutex
	httpClient  *http.Client
	githubAPIURL string
}

var log = ctrl.Log.WithName("providers.github")

// Name returns the provider name
func (p *GitHubProvider) Name() string {
	return p.name
}

// Type returns the provider type
func (p *GitHubProvider) Type() string {
	return "github"
}

// Init initializes the GitHub provider with options
func (p *GitHubProvider) Init(ctx context.Context, options map[string]interface{}) error {
	// Set default values
	p.githubAPIURL = "https://api.github.com/meta"
	p.enterprise = true
	p.cacheTTL = 15 * time.Minute
	p.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Process options
	if name, ok := options["name"].(string); ok {
		p.name = name
	} else {
		p.name = "github"
	}

	if enterprise, ok := options["enterprise"].(bool); ok {
		p.enterprise = enterprise
	}

	if !p.enterprise {
		log.Info("Using public GitHub IP ranges", "provider", p.name)
	} else {
		log.Info("Using GitHub Enterprise IP ranges", "provider", p.name)
	}

	if apiToken, ok := options["apiToken"].(string); ok {
		p.apiToken = apiToken
	}

	if cacheTTL, ok := options["cacheTTL"].(string); ok {
		duration, err := time.ParseDuration(cacheTTL)
		if err != nil {
			return fmt.Errorf("invalid cacheTTL format: %w", err)
		}
		p.cacheTTL = duration
	}

	if apiURL, ok := options["apiURL"].(string); ok && apiURL != "" {
		p.githubAPIURL = apiURL
	}

	log.Info("Initialized GitHub provider", 
		"name", p.name, 
		"enterprise", p.enterprise, 
		"cacheTTL", p.cacheTTL.String(),
		"apiURL", p.githubAPIURL)

	return nil
}

// FetchIPRanges fetches IP ranges from GitHub
func (p *GitHubProvider) FetchIPRanges(ctx context.Context) (*model.IPRangeSet, error) {
	// Check if we have a valid cache
	p.cacheMutex.RLock()
	if p.cachedData != nil && time.Since(p.lastFetch) < p.cacheTTL {
		defer p.cacheMutex.RUnlock()
		log.V(1).Info("Using cached GitHub IP ranges", "provider", p.name, "age", time.Since(p.lastFetch).String())
		return p.cachedData, nil
	}
	p.cacheMutex.RUnlock()

	// Lock for writing
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// Double-check cache under write lock
	if p.cachedData != nil && time.Since(p.lastFetch) < p.cacheTTL {
		log.V(1).Info("Using cached GitHub IP ranges (verified under lock)", "provider", p.name)
		return p.cachedData, nil
	}

	log.Info("Fetching GitHub IP ranges", "provider", p.name, "url", p.githubAPIURL)

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "GET", p.githubAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if p.apiToken != "" {
		req.Header.Set("Authorization", "token "+p.apiToken)
	}

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching GitHub IP ranges: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned non-OK status: %d, body: %s", resp.StatusCode, body)
	}

	// Read and parse the response
	var metadata GitHubIPMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("error decoding GitHub IP metadata: %w", err)
	}

	// Convert to our model
	ipRangeSet := model.NewIPRangeSet()

	// Process hooks
	for _, cidr := range metadata.Hooks {
		if err := ipRangeSet.Add(cidr, []string{"hooks"}); err != nil {
			log.Error(err, "Error adding hooks IP range", "cidr", cidr)
		}
	}

	// Process web
	for _, cidr := range metadata.Web {
		if err := ipRangeSet.Add(cidr, []string{"web"}); err != nil {
			log.Error(err, "Error adding web IP range", "cidr", cidr)
		}
	}

	// Process API
	for _, cidr := range metadata.API {
		if err := ipRangeSet.Add(cidr, []string{"api"}); err != nil {
			log.Error(err, "Error adding api IP range", "cidr", cidr)
		}
	}

	// Process Git
	for _, cidr := range metadata.Git {
		if err := ipRangeSet.Add(cidr, []string{"git"}); err != nil {
			log.Error(err, "Error adding git IP range", "cidr", cidr)
		}
	}

	// Process Pages
	for _, cidr := range metadata.Pages {
		if err := ipRangeSet.Add(cidr, []string{"pages"}); err != nil {
			log.Error(err, "Error adding pages IP range", "cidr", cidr)
		}
	}

	// Process Importer
	for _, cidr := range metadata.Importer {
		if err := ipRangeSet.Add(cidr, []string{"importer"}); err != nil {
			log.Error(err, "Error adding importer IP range", "cidr", cidr)
		}
	}

	// Process Actions
	for _, cidr := range metadata.Actions {
		if err := ipRangeSet.Add(cidr, []string{"actions"}); err != nil {
			log.Error(err, "Error adding actions IP range", "cidr", cidr)
		}
	}

	// Process Dependabot
	for _, cidr := range metadata.Dependabot {
		if err := ipRangeSet.Add(cidr, []string{"dependabot"}); err != nil {
			log.Error(err, "Error adding dependabot IP range", "cidr", cidr)
		}
	}

	log.Info("Successfully fetched GitHub IP ranges", 
		"provider", p.name, 
		"count", ipRangeSet.Count(),
		"web", len(metadata.Web),
		"api", len(metadata.API),
		"git", len(metadata.Git))

	// Update cache
	p.cachedData = ipRangeSet
	p.lastFetch = time.Now()

	return ipRangeSet, nil
}
