package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/galbakal/k8s-ingress-meta-sync/pkg/ingress"
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/model"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	ingress.Register("cloudflare", func() ingress.Ingress {
		return &CloudflareIngress{}
	})
}

// CloudflareIngress implements the Ingress interface for Cloudflare
type CloudflareIngress struct {
	name          string
	apiToken      string
	zoneID        string
	ruleName      string
	description   string
	action        string
	priority      int32
	updateStrategy string
	cacheTTL      time.Duration
	lastFetch     time.Time
	cachedData    *model.IPRangeSet
	cacheMutex    sync.RWMutex
	httpClient    *http.Client
}

var log = ctrl.Log.WithName("ingress.cloudflare")

// Name returns the ingress name
func (c *CloudflareIngress) Name() string {
	return c.name
}

// Type returns the ingress type
func (c *CloudflareIngress) Type() string {
	return "cloudflare"
}

// Init initializes the Cloudflare ingress with options
func (c *CloudflareIngress) Init(ctx context.Context, options map[string]interface{}) error {
	// Default values
	c.action = "allow"
	c.cacheTTL = 1 * time.Hour
	c.updateStrategy = "direct"
	c.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Process options
	if name, ok := options["name"].(string); ok {
		c.name = name
	} else {
		c.name = "cloudflare"
	}

	// Required options
	if apiToken, ok := options["apiToken"].(string); ok {
		c.apiToken = apiToken
	} else {
		return fmt.Errorf("apiToken is required")
	}

	if zoneID, ok := options["zoneId"].(string); ok {
		c.zoneID = zoneID
	} else {
		return fmt.Errorf("zoneId is required")
	}

	if ruleName, ok := options["ruleName"].(string); ok {
		c.ruleName = ruleName
	} else {
		return fmt.Errorf("ruleName is required")
	}

	// Optional options
	if description, ok := options["description"].(string); ok {
		c.description = description
	} else {
		c.description = "Auto-managed IP ranges by k8s-ingress-meta-sync"
	}

	if action, ok := options["action"].(string); ok {
		c.action = action
	}

	if priority, ok := options["priority"].(int32); ok {
		c.priority = priority
	}

	if updateStrategy, ok := options["updateStrategy"].(string); ok {
		c.updateStrategy = updateStrategy
	}

	if cacheTTL, ok := options["cacheTTL"].(string); ok {
		duration, err := time.ParseDuration(cacheTTL)
		if err != nil {
			return fmt.Errorf("invalid cacheTTL format: %w", err)
		}
		c.cacheTTL = duration
	}

	log.Info("Initialized Cloudflare ingress", 
		"name", c.name, 
		"zoneID", c.zoneID, 
		"ruleName", c.ruleName,
		"updateStrategy", c.updateStrategy)

	return nil
}

// CloudflareRule represents a Cloudflare firewall rule
type CloudflareRule struct {
	ID          string `json:"id,omitempty"`
	Paused      bool   `json:"paused"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Priority    int32  `json:"priority,omitempty"`
	Filter      Filter `json:"filter"`
}

// Filter represents a Cloudflare firewall filter
type Filter struct {
	ID          string `json:"id,omitempty"`
	Expression  string `json:"expression"`
	Paused      bool   `json:"paused"`
	Description string `json:"description"`
}

// CloudflareResponse represents a response from the Cloudflare API
type CloudflareResponse struct {
	Success  bool              `json:"success"`
	Errors   []CloudflareError `json:"errors"`
	Messages []string          `json:"messages"`
	Result   json.RawMessage   `json:"result"`
}

// CloudflareError represents an error from the Cloudflare API
type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// GetCurrentIPRanges gets the current IP ranges configured in Cloudflare
func (c *CloudflareIngress) GetCurrentIPRanges(ctx context.Context) (*model.IPRangeSet, error) {
	// Check if we have a valid cache
	c.cacheMutex.RLock()
	if c.cachedData != nil && time.Since(c.lastFetch) < c.cacheTTL {
		defer c.cacheMutex.RUnlock()
		log.V(1).Info("Using cached Cloudflare IP ranges", "ingress", c.name, "age", time.Since(c.lastFetch).String())
		return c.cachedData, nil
	}
	c.cacheMutex.RUnlock()

	// Lock for writing
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	// Double-check cache under write lock
	if c.cachedData != nil && time.Since(c.lastFetch) < c.cacheTTL {
		log.V(1).Info("Using cached Cloudflare IP ranges (verified under lock)", "ingress", c.name)
		return c.cachedData, nil
	}

	// First, get all filters to find our filter
	filterID, err := c.findFilterID(ctx)
	if err != nil {
		return nil, err
	}

	if filterID == "" {
		// Filter doesn't exist yet, return empty set
		log.Info("Filter doesn't exist yet", "ingress", c.name, "ruleName", c.ruleName)
		emptySet := model.NewIPRangeSet()
		c.cachedData = emptySet
		c.lastFetch = time.Now()
		return emptySet, nil
	}

	// Get the filter details
	filterDetails, err := c.getFilterDetails(ctx, filterID)
	if err != nil {
		return nil, err
	}

	// Parse the filter expression to extract IP ranges
	ipRanges := c.parseFilterExpression(filterDetails.Expression)

	log.Info("Got current Cloudflare IP ranges", "ingress", c.name, "count", ipRanges.Count())

	// Update cache
	c.cachedData = ipRanges
	c.lastFetch = time.Now()

	return ipRanges, nil
}

// parseFilterExpression parses a Cloudflare filter expression to extract IP ranges
func (c *CloudflareIngress) parseFilterExpression(expression string) *model.IPRangeSet {
	// Example expression: (ip.src in {1.2.3.4/32 5.6.7.8/32})
	// We need to extract the CIDRs
	
	ipRangeSet := model.NewIPRangeSet()
	
	// Very simple parser for now - this could be improved
	// This assumes the expression is in the format (ip.src in {cidr1 cidr2 ...})
	
	// Find the opening brace
	openBrace := 0
	for i, char := range expression {
		if char == '{' {
			openBrace = i
			break
		}
	}
	
	// Find the closing brace
	closeBrace := 0
	for i := len(expression) - 1; i >= 0; i-- {
		if expression[i] == '}' {
			closeBrace = i
			break
		}
	}
	
	if openBrace == 0 || closeBrace == 0 || openBrace >= closeBrace {
		log.Error(fmt.Errorf("invalid filter expression format"), "Failed to parse filter expression", "expression", expression)
		return ipRangeSet
	}
	
	// Extract the CIDRs
	cidrsStr := expression[openBrace+1:closeBrace]
	cidrs := make([]string, 0)
	
	// Split by space
	for _, cidr := range bytes.Fields([]byte(cidrsStr)) {
		cidrs = append(cidrs, string(cidr))
	}
	
	// Add to the IP range set
	for _, cidr := range cidrs {
		if err := ipRangeSet.Add(cidr, []string{"cloudflare"}); err != nil {
			log.Error(err, "Error adding CIDR to IP range set", "cidr", cidr)
		}
	}
	
	return ipRangeSet
}

// findFilterID finds the filter ID for the given rule name
func (c *CloudflareIngress) findFilterID(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/filters", c.zoneID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error getting filters: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, body)
	}
	
	var response CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}
	
	if !response.Success {
		return "", fmt.Errorf("API request was not successful: %v", response.Errors)
	}
	
	var filters []Filter
	if err := json.Unmarshal(response.Result, &filters); err != nil {
		return "", fmt.Errorf("error unmarshaling filters: %w", err)
	}
	
	// Find the filter with the matching description
	for _, filter := range filters {
		if filter.Description == c.ruleName {
			return filter.ID, nil
		}
	}
	
	// Filter not found
	return "", nil
}

// getFilterDetails gets the details of a filter
func (c *CloudflareIngress) getFilterDetails(ctx context.Context, filterID string) (*Filter, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/filters/%s", c.zoneID, filterID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting filter details: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, body)
	}
	
	var response CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	
	if !response.Success {
		return nil, fmt.Errorf("API request was not successful: %v", response.Errors)
	}
	
	var filter Filter
	if err := json.Unmarshal(response.Result, &filter); err != nil {
		return nil, fmt.Errorf("error unmarshaling filter: %w", err)
	}
	
	return &filter, nil
}

// ApplyIPRanges applies the given IP ranges to Cloudflare
func (c *CloudflareIngress) ApplyIPRanges(ctx context.Context, ipRanges *model.IPRangeSet) error {
	log.Info("Applying IP ranges to Cloudflare", "ingress", c.name, "count", ipRanges.Count())
	
	// Get current IP ranges for diff
	currentRanges, err := c.GetCurrentIPRanges(ctx)
	if err != nil {
		return fmt.Errorf("error getting current IP ranges: %w", err)
	}
	
	added, removed := currentRanges.Diff(ipRanges)
	log.Info("IP range diff", "ingress", c.name, "added", added.Count(), "removed", removed.Count())
	
	// If no changes, we're done
	if added.Count() == 0 && removed.Count() == 0 {
		log.Info("No changes to apply", "ingress", c.name)
		return nil
	}
	
	// Based on update strategy
	if c.updateStrategy == "direct" || currentRanges.Count() == 0 {
		// Direct update - replace the entire rule
		return c.createOrUpdateRule(ctx, ipRanges)
	} else if c.updateStrategy == "incremental" {
		// Incremental update - add/remove specific IPs
		// Note: This would require implementing more complex logic to update the existing rule
		// For simplicity, we'll just do the direct update for now
		return c.createOrUpdateRule(ctx, ipRanges)
	}
	
	return fmt.Errorf("unknown update strategy: %s", c.updateStrategy)
}

// createOrUpdateRule creates or updates a Cloudflare firewall rule
func (c *CloudflareIngress) createOrUpdateRule(ctx context.Context, ipRanges *model.IPRangeSet) error {
	// Check if rule already exists
	filterID, err := c.findFilterID(ctx)
	if err != nil {
		return fmt.Errorf("error finding filter ID: %w", err)
	}
	
	// Build the expression
	expression := c.buildFilterExpression(ipRanges)
	
	if filterID == "" {
		// Create new rule
		log.Info("Creating new Cloudflare rule", "ingress", c.name, "ruleName", c.ruleName)
		return c.createRule(ctx, expression)
	}
	
	// Update existing rule
	log.Info("Updating existing Cloudflare rule", "ingress", c.name, "ruleName", c.ruleName, "filterID", filterID)
	return c.updateRule(ctx, filterID, expression)
}

// buildFilterExpression builds a Cloudflare filter expression from IP ranges
func (c *CloudflareIngress) buildFilterExpression(ipRanges *model.IPRangeSet) string {
	cidrs := ipRanges.GetCIDRs()
	
	if len(cidrs) == 0 {
		// Empty rule
		return "(ip.src eq 0.0.0.0/32)"
	}
	
	// Combine all CIDRs
	cidrList := ""
	for i, cidr := range cidrs {
		if i > 0 {
			cidrList += " "
		}
		cidrList += cidr
	}
	
	return fmt.Sprintf("(ip.src in {%s})", cidrList)
}

// createRule creates a new Cloudflare firewall rule
func (c *CloudflareIngress) createRule(ctx context.Context, expression string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/filters", c.zoneID)
	
	// Create filter first
	filter := Filter{
		Expression:  expression,
		Paused:      false,
		Description: c.ruleName,
	}
	
	filterData, err := json.Marshal([]Filter{filter})
	if err != nil {
		return fmt.Errorf("error marshaling filter: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(filterData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error creating filter: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, body)
	}
	
	var response CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	
	if !response.Success {
		return fmt.Errorf("API request was not successful: %v", response.Errors)
	}
	
	var filters []Filter
	if err := json.Unmarshal(response.Result, &filters); err != nil {
		return fmt.Errorf("error unmarshaling filters: %w", err)
	}
	
	if len(filters) == 0 {
		return fmt.Errorf("no filter was created")
	}
	
	filterID := filters[0].ID
	
	// Now create the rule
	return c.createFirewallRule(ctx, filterID)
}

// createFirewallRule creates a new Cloudflare firewall rule
func (c *CloudflareIngress) createFirewallRule(ctx context.Context, filterID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/firewall/rules", c.zoneID)
	
	rule := CloudflareRule{
		Filter: Filter{
			ID: filterID,
		},
		Action:      c.action,
		Description: c.description,
		Paused:      false,
	}
	
	if c.priority != 0 {
		rule.Priority = c.priority
	}
	
	ruleData, err := json.Marshal([]CloudflareRule{rule})
	if err != nil {
		return fmt.Errorf("error marshaling rule: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(ruleData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error creating firewall rule: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, body)
	}
	
	var response CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	
	if !response.Success {
		return fmt.Errorf("API request was not successful: %v", response.Errors)
	}
	
	// Update cache
	ipRangeSet := model.NewIPRangeSet()
	for _, cidr := range c.parseFilterExpression(rule.Filter.Expression).GetCIDRs() {
		if err := ipRangeSet.Add(cidr, []string{"cloudflare"}); err != nil {
			log.Error(err, "Error adding CIDR to IP range set", "cidr", cidr)
		}
	}
	
	c.cacheMutex.Lock()
	c.cachedData = ipRangeSet
	c.lastFetch = time.Now()
	c.cacheMutex.Unlock()
	
	log.Info("Successfully created Cloudflare rule", "ingress", c.name, "ruleID", rule.ID)
	
	return nil
}

// updateRule updates an existing Cloudflare firewall rule
func (c *CloudflareIngress) updateRule(ctx context.Context, filterID string, expression string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/filters/%s", c.zoneID, filterID)
	
	filter := Filter{
		ID:         filterID,
		Expression: expression,
		Paused:     false,
	}
	
	filterData, err := json.Marshal(filter)
	if err != nil {
		return fmt.Errorf("error marshaling filter: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(filterData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error updating filter: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, body)
	}
	
	var response CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	
	if !response.Success {
		return fmt.Errorf("API request was not successful: %v", response.Errors)
	}
	
	// Update cache
	ipRangeSet := model.NewIPRangeSet()
	cidrs := c.parseFilterExpression(expression).GetCIDRs()
	for _, cidr := range cidrs {
		if err := ipRangeSet.Add(cidr, []string{"cloudflare"}); err != nil {
			log.Error(err, "Error adding CIDR to IP range set", "cidr", cidr)
		}
	}
	
	c.cacheMutex.Lock()
	c.cachedData = ipRangeSet
	c.lastFetch = time.Now()
	c.cacheMutex.Unlock()
	
	log.Info("Successfully updated Cloudflare rule", "ingress", c.name, "filterID", filterID, "cidrCount", len(cidrs))
	
	return nil
}
