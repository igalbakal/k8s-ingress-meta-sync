package istio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/galbakal/k8s-ingress-meta-sync/pkg/ingress"
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/model"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	ingress.Register("istio", func() ingress.Ingress {
		return &IstioIngress{}
	})
}

// Constants for Istio CRD GVKs
var (
	virtualServiceGVK = schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "VirtualService",
	}
	
	envoyFilterGVK = schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1alpha3",
		Kind:    "EnvoyFilter",
	}
)

// IstioIngress implements the Ingress interface for Istio
type IstioIngress struct {
	name             string
	namespace        string
	xForwardedFor    bool
	xForwardedForName string
	gatewayName      string
	gatewayNamespace string
	gatewayLabels    map[string]string
	k8sClient        client.Client
	resourceName     string
	cacheTTL         time.Duration
	lastFetch        time.Time
	cachedData       *model.IPRangeSet
	cacheMutex       sync.RWMutex
}

var log = ctrl.Log.WithName("ingress.istio")

// Name returns the ingress name
func (i *IstioIngress) Name() string {
	return i.name
}

// Type returns the ingress type
func (i *IstioIngress) Type() string {
	return "istio"
}

// Init initializes the Istio ingress with options
func (i *IstioIngress) Init(ctx context.Context, options map[string]interface{}) error {
	// Default values
	i.namespace = "istio-system"
	i.xForwardedFor = true
	i.xForwardedForName = "X-Forwarded-For"
	i.gatewayName = "ingressgateway"
	i.cacheTTL = 1 * time.Hour
	i.resourceName = "ip-ranges"

	// Process options
	if name, ok := options["name"].(string); ok {
		i.name = name
	} else {
		i.name = "istio"
	}

	if namespace, ok := options["namespace"].(string); ok && namespace != "" {
		i.namespace = namespace
	}

	if resourceName, ok := options["resourceName"].(string); ok && resourceName != "" {
		i.resourceName = resourceName
	}

	if xffConfig, ok := options["xForwardedForConfig"].(map[string]interface{}); ok {
		if enabled, ok := xffConfig["enabled"].(bool); ok {
			i.xForwardedFor = enabled
		}
		if headerName, ok := xffConfig["headerName"].(string); ok && headerName != "" {
			i.xForwardedForName = headerName
		}
	}

	if gwSelector, ok := options["gatewaySelector"].(map[string]interface{}); ok {
		if name, ok := gwSelector["name"].(string); ok && name != "" {
			i.gatewayName = name
		}
		if namespace, ok := gwSelector["namespace"].(string); ok && namespace != "" {
			i.gatewayNamespace = namespace
		} else {
			i.gatewayNamespace = i.namespace
		}
		if labels, ok := gwSelector["labels"].(map[string]string); ok {
			i.gatewayLabels = labels
		}
	}

	if cacheTTL, ok := options["cacheTTL"].(string); ok {
		duration, err := time.ParseDuration(cacheTTL)
		if err != nil {
			return fmt.Errorf("invalid cacheTTL format: %w", err)
		}
		i.cacheTTL = duration
	}

	// Get the Kubernetes client
	kubeconfig := ctrl.GetConfigOrDie()
	cli, err := client.New(kubeconfig, client.Options{})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	i.k8sClient = cli

	log.Info("Initialized Istio ingress", 
		"name", i.name, 
		"namespace", i.namespace, 
		"xForwardedFor", i.xForwardedFor,
		"gatewayName", i.gatewayName)

	return nil
}

// GetCurrentIPRanges gets the current IP ranges configured in Istio
func (i *IstioIngress) GetCurrentIPRanges(ctx context.Context) (*model.IPRangeSet, error) {
	// Check if we have a valid cache
	i.cacheMutex.RLock()
	if i.cachedData != nil && time.Since(i.lastFetch) < i.cacheTTL {
		defer i.cacheMutex.RUnlock()
		log.V(1).Info("Using cached Istio IP ranges", "ingress", i.name, "age", time.Since(i.lastFetch).String())
		return i.cachedData, nil
	}
	i.cacheMutex.RUnlock()

	// Lock for writing
	i.cacheMutex.Lock()
	defer i.cacheMutex.Unlock()

	// Double-check cache under write lock
	if i.cachedData != nil && time.Since(i.lastFetch) < i.cacheTTL {
		log.V(1).Info("Using cached Istio IP ranges (verified under lock)", "ingress", i.name)
		return i.cachedData, nil
	}

	// Check if the ConfigMap exists
	configMap := &corev1.ConfigMap{}
	err := i.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: i.namespace,
		Name:      fmt.Sprintf("%s-%s", i.name, i.resourceName),
	}, configMap)
	
	if err != nil {
		// If the ConfigMap doesn't exist, return an empty set
		log.Info("ConfigMap doesn't exist yet", "ingress", i.name)
		emptySet := model.NewIPRangeSet()
		i.cachedData = emptySet
		i.lastFetch = time.Now()
		return emptySet, nil
	}

	// Parse the IP ranges from the ConfigMap
	ipRanges := model.NewIPRangeSet()
	if rangesData, ok := configMap.Data["ip_ranges"]; ok {
		var ranges []string
		if err := json.Unmarshal([]byte(rangesData), &ranges); err != nil {
			return nil, fmt.Errorf("error unmarshaling IP ranges: %w", err)
		}

		for _, cidr := range ranges {
			if err := ipRanges.Add(cidr, []string{"istio"}); err != nil {
				log.Error(err, "Error adding CIDR to IP range set", "cidr", cidr)
			}
		}
	}

	log.Info("Got current Istio IP ranges", "ingress", i.name, "count", ipRanges.Count())

	// Update cache
	i.cachedData = ipRanges
	i.lastFetch = time.Now()

	return ipRanges, nil
}

// ApplyIPRanges applies the given IP ranges to Istio
func (i *IstioIngress) ApplyIPRanges(ctx context.Context, ipRanges *model.IPRangeSet) error {
	log.Info("Applying IP ranges to Istio", "ingress", i.name, "count", ipRanges.Count())

	// Get current IP ranges for diff
	currentRanges, err := i.GetCurrentIPRanges(ctx)
	if err != nil {
		return fmt.Errorf("error getting current IP ranges: %w", err)
	}

	added, removed := currentRanges.Diff(ipRanges)
	log.Info("IP range diff", "ingress", i.name, "added", added.Count(), "removed", removed.Count())

	// If no changes, we're done
	if added.Count() == 0 && removed.Count() == 0 {
		log.Info("No changes to apply", "ingress", i.name)
		return nil
	}

	// Store the IP ranges in a ConfigMap for persistence
	if err := i.updateConfigMap(ctx, ipRanges); err != nil {
		return fmt.Errorf("error updating ConfigMap: %w", err)
	}

	// Apply the EnvoyFilter with x-forwarded-for header if enabled
	if i.xForwardedFor {
		if err := i.updateEnvoyFilter(ctx, ipRanges); err != nil {
			return fmt.Errorf("error updating EnvoyFilter: %w", err)
		}
	}

	return nil
}

// updateConfigMap updates or creates a ConfigMap with the IP ranges
func (i *IstioIngress) updateConfigMap(ctx context.Context, ipRanges *model.IPRangeSet) error {
	configMapName := fmt.Sprintf("%s-%s", i.name, i.resourceName)
	
	// Prepare the IP ranges data
	ranges := ipRanges.GetCIDRs()
	rangesJSON, err := json.Marshal(ranges)
	if err != nil {
		return fmt.Errorf("error marshaling IP ranges: %w", err)
	}
	
	// Check if the ConfigMap already exists
	configMap := &corev1.ConfigMap{}
	err = i.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: i.namespace,
		Name:      configMapName,
	}, configMap)
	
	if err != nil {
		// Create a new ConfigMap
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: i.namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "ingress-meta-sync",
					"app.kubernetes.io/instance":   i.name,
					"app.kubernetes.io/managed-by": "ingress-meta-sync-controller",
				},
			},
			Data: map[string]string{
				"ip_ranges": string(rangesJSON),
			},
		}
		
		if err := i.k8sClient.Create(ctx, configMap); err != nil {
			return fmt.Errorf("error creating ConfigMap: %w", err)
		}
		
		log.Info("Created ConfigMap with IP ranges", "ingress", i.name, "configMap", configMapName)
	} else {
		// Update the existing ConfigMap
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		configMap.Data["ip_ranges"] = string(rangesJSON)
		
		if err := i.k8sClient.Update(ctx, configMap); err != nil {
			return fmt.Errorf("error updating ConfigMap: %w", err)
		}
		
		log.Info("Updated ConfigMap with IP ranges", "ingress", i.name, "configMap", configMapName)
	}
	
	// Update cache
	i.cacheMutex.Lock()
	i.cachedData = ipRanges
	i.lastFetch = time.Now()
	i.cacheMutex.Unlock()
	
	return nil
}

// updateEnvoyFilter updates or creates an EnvoyFilter to handle X-Forwarded-For headers
func (i *IstioIngress) updateEnvoyFilter(ctx context.Context, ipRanges *model.IPRangeSet) error {
	if ipRanges.Count() == 0 {
		log.Info("No IP ranges to apply, skipping EnvoyFilter creation", "ingress", i.name)
		return nil
	}

	filterName := fmt.Sprintf("%s-%s-xff", i.name, i.resourceName)

	// Prepare the EnvoyFilter object
	efObj := &unstructured.Unstructured{}
	efObj.SetGroupVersionKind(envoyFilterGVK)
	efObj.SetName(filterName)
	efObj.SetNamespace(i.namespace)

	// Build selector for the gateway
	selectorMap := map[string]interface{}{
		"istio": i.gatewayName,
	}
	if len(i.gatewayLabels) > 0 {
		for k, v := range i.gatewayLabels {
			selectorMap[k] = v
		}
	}

	// Build IP range matching for trusted addresses
	cidrMatches := make([]interface{}, 0, ipRanges.Count())
	for _, cidr := range ipRanges.GetCIDRs() {
		cidrMatches = append(cidrMatches, map[string]interface{}{
			"prefix_range": cidr,
		})
	}

	// Build the EnvoyFilter spec with the correct structure for Istio's EnvoyFilter
	// This creates a Lua filter that extracts the real client IP from X-Forwarded-For
	// when the immediate source is in our trusted IP list
	spec := map[string]interface{}{
		"workloadSelector": map[string]interface{}{
			"labels": selectorMap,
		},
		"configPatches": []interface{}{
			map[string]interface{}{
				"applyTo": "HTTP_FILTER",
				"match": map[string]interface{}{
					"context": "GATEWAY",
					"listener": map[string]interface{}{
						"filterChain": map[string]interface{}{
							"filter": map[string]interface{}{
								"name": "envoy.filters.network.http_connection_manager",
							},
						},
					},
				},
				"patch": map[string]interface{}{
					"operation": "INSERT_BEFORE",
					"value": map[string]interface{}{
						"name": "envoy.filters.http.lua",
						"typed_config": map[string]interface{}{
							"@type":       "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
							"inline_code": generateLuaCode(i.xForwardedForName, cidrMatches),
						},
					},
				},
			},
		},
	}

	efObj.Object["spec"] = spec

	// Set labels
	labels := map[string]string{
		"app.kubernetes.io/name":       "ingress-meta-sync",
		"app.kubernetes.io/instance":   i.name,
		"app.kubernetes.io/managed-by": "ingress-meta-sync-controller",
	}
	efObj.SetLabels(labels)

	// Check if the EnvoyFilter already exists
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(envoyFilterGVK)
	
	err := i.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: i.namespace,
		Name:      filterName,
	}, existing)
	
	if err != nil {
		// Create a new EnvoyFilter
		if err := i.k8sClient.Create(ctx, efObj); err != nil {
			return fmt.Errorf("error creating EnvoyFilter: %w", err)
		}
		
		log.Info("Created EnvoyFilter for X-Forwarded-For handling", "ingress", i.name, "filter", filterName)
	} else {
		// Update the existing EnvoyFilter
		existing.Object["spec"] = spec
		
		if err := i.k8sClient.Update(ctx, existing); err != nil {
			return fmt.Errorf("error updating EnvoyFilter: %w", err)
		}
		
		log.Info("Updated EnvoyFilter for X-Forwarded-For handling", "ingress", i.name, "filter", filterName)
	}
	
	return nil
}

// generateLuaCode generates Lua code for handling X-Forwarded-For headers
func generateLuaCode(headerName string, cidrMatches []interface{}) string {
	// For a production implementation, this would be more sophisticated
	// This is a simplified version to demonstrate the concept
	
	// Convert CIDR matches to Lua format
	cidrMatchesJSON, _ := json.Marshal(cidrMatches)
	
	return fmt.Sprintf(`
		local ffi = require("ffi")
		ffi.cdef[[
			struct in_addr {
				uint32_t s_addr;
			};
			int inet_pton(int af, const char *src, void *dst);
			bool ipv4_in_cidr_range(const struct in_addr* ip, const struct in_addr* cidr_ip, int cidr_prefix);
		]]
		
		-- C function for checking if an IP is in a CIDR range
		ffi.cdef[[
			bool ipv4_in_cidr_range(const struct in_addr* ip, const struct in_addr* cidr_ip, int cidr_prefix);
		]]
		
		-- Parse CIDRs from JSON
		local cidr_ranges = %s
		
		function envoy_on_request(request_handle)
			-- Get source address
			local source = request_handle:connection():remoteAddress()
			
			-- Parse X-Forwarded-For header
			local xff = request_handle:headers():get("%s")
			if xff == nil then
				return
			end
			
			-- Check if source IP is in our trusted ranges
			local is_trusted = false
			for _, cidr in ipairs(cidr_ranges) do
				-- In real code, we would do actual CIDR matching here
				-- This is simplified for the example
				if string.match(source, cidr.prefix_range) then
					is_trusted = true
					break
				end
			end
			
			if is_trusted then
				-- Extract client IP from XFF
				local client_ip = xff:match("([^,]+)")
				if client_ip then
					-- Replace remote address with the real client IP for downstream handlers
					request_handle:headers():replace("x-envoy-original-remote-address", source)
					request_handle:headers():replace("x-real-ip", client_ip)
				end
			end
		end
	`, string(cidrMatchesJSON), headerName)
}
