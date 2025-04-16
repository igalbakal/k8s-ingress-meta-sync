package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ingressmetasyncv1alpha1 "github.com/galbakal/k8s-ingress-meta-sync/pkg/apis/ingressmetasync/v1alpha1"
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/ingress"
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/model"
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/providers"
)

// SyncReconciler reconciles a SyncConfig object
type SyncReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	ProviderCache   map[string]providers.Provider
	IngressCache    map[string]ingress.Ingress
	SecretReader    SecretReader
}

// SecretReader is an interface for reading secrets
type SecretReader interface {
	GetSecret(ctx context.Context, namespace, name, key string) (string, error)
}

// DefaultSecretReader is the default implementation of SecretReader
type DefaultSecretReader struct {
	Client client.Client
}

// GetSecret gets a secret value from the given secret name, namespace, and key
func (r *DefaultSecretReader) GetSecret(ctx context.Context, namespace, name, key string) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return "", err
	}

	if data, ok := secret.Data[key]; ok {
		return string(data), nil
	}

	// If key isn't specified or not found, try to use the data directly
	if len(secret.Data) == 1 {
		// If there's only one key, use that
		for _, data := range secret.Data {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("key %s not found in secret %s/%s", key, namespace, name)
}

// SetupWithManager sets up the controller with the Manager
func SetupWithManager(mgr manager.Manager) error {
	r := &SyncReconciler{
		Client:          mgr.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("SyncConfig"),
		Scheme:          mgr.GetScheme(),
		Recorder:        mgr.GetEventRecorderFor("syncconfig-controller"),
		ProviderCache:   make(map[string]providers.Provider),
		IngressCache:    make(map[string]ingress.Ingress),
		SecretReader:    &DefaultSecretReader{Client: mgr.GetClient()},
	}

	// Watch for changes to SyncConfig
	return ctrl.NewControllerManagedBy(mgr).
		For(&ingressmetasyncv1alpha1.SyncConfig{}).
		Watches(
			&source.Kind{Type: &ingressmetasyncv1alpha1.ProviderConfig{}},
			handler.EnqueueRequestsFromMapFunc(r.findSyncConfigsForProvider),
		).
		Watches(
			&source.Kind{Type: &ingressmetasyncv1alpha1.IngressConfig{}},
			handler.EnqueueRequestsFromMapFunc(r.findSyncConfigsForIngress),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 4, // Limit concurrent reconciliations
		}).
		Complete(r)
}

// findSyncConfigsForProvider maps a ProviderConfig to SyncConfigs that reference it
func (r *SyncReconciler) findSyncConfigsForProvider(providerObj client.Object) []reconcile.Request {
	provider := providerObj.(*ingressmetasyncv1alpha1.ProviderConfig)
	var syncConfigs ingressmetasyncv1alpha1.SyncConfigList
	err := r.List(context.Background(), &syncConfigs)
	if err != nil {
		r.Log.Error(err, "Unable to list SyncConfigs")
		return nil
	}

	var requests []reconcile.Request
	for _, sync := range syncConfigs.Items {
		for _, providerRef := range sync.Spec.Providers {
			if providerRef.Name == provider.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: sync.Name,
					},
				})
				break
			}
		}
	}
	return requests
}

// findSyncConfigsForIngress maps an IngressConfig to SyncConfigs that reference it
func (r *SyncReconciler) findSyncConfigsForIngress(ingressObj client.Object) []reconcile.Request {
	ingressConfig := ingressObj.(*ingressmetasyncv1alpha1.IngressConfig)
	var syncConfigs ingressmetasyncv1alpha1.SyncConfigList
	err := r.List(context.Background(), &syncConfigs)
	if err != nil {
		r.Log.Error(err, "Unable to list SyncConfigs")
		return nil
	}

	var requests []reconcile.Request
	for _, sync := range syncConfigs.Items {
		for _, ingressRef := range sync.Spec.Ingress {
			if ingressRef.Name == ingressConfig.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: sync.Name,
					},
				})
				break
			}
		}
	}
	return requests
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *SyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("syncconfig", req.NamespacedName)
	log.Info("Reconciling SyncConfig")

	// Fetch the SyncConfig instance
	var syncConfig ingressmetasyncv1alpha1.SyncConfig
	if err := r.Get(ctx, req.NamespacedName, &syncConfig); err != nil {
		if errors.IsNotFound(err) {
			// SyncConfig was deleted, clean up any resources if needed
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to fetch SyncConfig")
		return ctrl.Result{}, err
	}

	// Initialize status if it's the first reconciliation
	if syncConfig.Status.Conditions == nil {
		syncConfig.Status.Conditions = []metav1.Condition{}
	}

	// Prepare for status updates
	syncConfig.Status.LastSyncTime = &metav1.Time{Time: time.Now()}

	// Collect IP ranges from all providers
	allRanges, err := r.collectProviderIPRanges(ctx, &syncConfig)
	if err != nil {
		r.setStatusCondition(&syncConfig, "Ready", metav1.ConditionFalse, "ProviderError", err.Error())
		if err := r.Status().Update(ctx, &syncConfig); err != nil {
			log.Error(err, "Unable to update SyncConfig status")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
	}

	// Apply IP ranges to all ingress services
	if err := r.applyIPRangesToIngress(ctx, &syncConfig, allRanges); err != nil {
		r.setStatusCondition(&syncConfig, "Ready", metav1.ConditionFalse, "IngressError", err.Error())
		if err := r.Status().Update(ctx, &syncConfig); err != nil {
			log.Error(err, "Unable to update SyncConfig status")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
	}

	// Update status to show sync was successful
	syncConfig.Status.LastSuccessfulSync = &metav1.Time{Time: time.Now()}
	r.setStatusCondition(&syncConfig, "Ready", metav1.ConditionTrue, "SyncSuccessful", "Successfully synced IP ranges")

	// Update the status
	if err := r.Status().Update(ctx, &syncConfig); err != nil {
		log.Error(err, "Unable to update SyncConfig status")
		return ctrl.Result{}, err
	}

	// Requeue for next sync - base on the lowest polling interval
	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// setStatusCondition sets a condition in the status
func (r *SyncReconciler) setStatusCondition(syncConfig *ingressmetasyncv1alpha1.SyncConfig, conditionType string, 
	status metav1.ConditionStatus, reason, message string) {
	
	// Find the condition if it already exists
	var existingCondition *metav1.Condition
	for i := range syncConfig.Status.Conditions {
		if syncConfig.Status.Conditions[i].Type == conditionType {
			existingCondition = &syncConfig.Status.Conditions[i]
			break
		}
	}

	// If it doesn't exist, append a new one
	if existingCondition == nil {
		syncConfig.Status.Conditions = append(syncConfig.Status.Conditions, metav1.Condition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: metav1.Time{Time: time.Now()},
			Reason:             reason,
			Message:            message,
		})
		return
	}

	// Update existing condition
	if existingCondition.Status != status {
		existingCondition.Status = status
		existingCondition.LastTransitionTime = metav1.Time{Time: time.Now()}
	}
	existingCondition.Reason = reason
	existingCondition.Message = message
}

// collectProviderIPRanges collects IP ranges from all providers in the SyncConfig
func (r *SyncReconciler) collectProviderIPRanges(ctx context.Context, syncConfig *ingressmetasyncv1alpha1.SyncConfig) (*model.IPRangeSet, error) {
	allRanges := model.NewIPRangeSet()
	updatedProviderStatus := make([]ingressmetasyncv1alpha1.ProviderSyncStatus, 0)

	for _, providerRef := range syncConfig.Spec.Providers {
		providerStatus := ingressmetasyncv1alpha1.ProviderSyncStatus{
			Name:         providerRef.Name,
			LastSyncTime: &metav1.Time{Time: time.Now()},
			Status:       "Pending",
		}

		// Fetch the ProviderConfig
		var providerConfig ingressmetasyncv1alpha1.ProviderConfig
		if err := r.Get(ctx, types.NamespacedName{Name: providerRef.Name}, &providerConfig); err != nil {
			errMsg := fmt.Sprintf("Unable to fetch ProviderConfig: %v", err)
			providerStatus.Error = errMsg
			providerStatus.Status = "Error"
			updatedProviderStatus = append(updatedProviderStatus, providerStatus)
			
			if syncConfig.Spec.SyncPolicy != nil && syncConfig.Spec.SyncPolicy.FailureMode == "fail" {
				return nil, fmt.Errorf(errMsg)
			}
			
			r.Log.Error(err, "Unable to fetch ProviderConfig", "provider", providerRef.Name)
			continue
		}

		// Get the provider instance from cache or create a new one
		providerInstance, err := r.getOrCreateProvider(ctx, &providerConfig)
		if err != nil {
			errMsg := fmt.Sprintf("Unable to create provider: %v", err)
			providerStatus.Error = errMsg
			providerStatus.Status = "Error"
			updatedProviderStatus = append(updatedProviderStatus, providerStatus)
			
			if syncConfig.Spec.SyncPolicy != nil && syncConfig.Spec.SyncPolicy.FailureMode == "fail" {
				return nil, fmt.Errorf(errMsg)
			}
			
			r.Log.Error(err, "Unable to create provider", "provider", providerRef.Name)
			continue
		}

		// Fetch IP ranges from the provider
		providerRanges, err := providerInstance.FetchIPRanges(ctx)
		if err != nil {
			errMsg := fmt.Sprintf("Unable to fetch IP ranges: %v", err)
			providerStatus.Error = errMsg
			providerStatus.Status = "Error"
			updatedProviderStatus = append(updatedProviderStatus, providerStatus)
			
			if syncConfig.Spec.SyncPolicy != nil && syncConfig.Spec.SyncPolicy.FailureMode == "fail" {
				return nil, fmt.Errorf(errMsg)
			}
			
			r.Log.Error(err, "Unable to fetch IP ranges", "provider", providerRef.Name)
			continue
		}

		// Filter ranges if include/exclude are specified
		filteredRanges := providerRanges
		if len(providerRef.IncludeRanges) > 0 || len(providerRef.ExcludeRanges) > 0 {
			filteredRanges = providerRanges.Filter(providerRef.IncludeRanges, providerRef.ExcludeRanges)
			r.Log.Info("Filtered IP ranges", 
				"provider", providerRef.Name, 
				"before", providerRanges.Count(), 
				"after", filteredRanges.Count(),
				"includeFilters", providerRef.IncludeRanges,
				"excludeFilters", providerRef.ExcludeRanges)
		}

		// Merge with all ranges
		allRanges = allRanges.Merge(filteredRanges)

		// Update provider status
		providerStatus.Status = "Success"
		providerStatus.IPRangesCount = int32(filteredRanges.Count())
		updatedProviderStatus = append(updatedProviderStatus, providerStatus)
	}

	// Update the SyncConfig status with provider status
	syncConfig.Status.ProviderStatus = updatedProviderStatus

	return allRanges, nil
}

// getOrCreateProvider gets an existing provider from cache or creates a new one
func (r *SyncReconciler) getOrCreateProvider(ctx context.Context, providerConfig *ingressmetasyncv1alpha1.ProviderConfig) (providers.Provider, error) {
	// Check if we already have this provider in the cache
	if provider, ok := r.ProviderCache[providerConfig.Name]; ok {
		return provider, nil
	}

	// Create a new provider instance
	providerInstance, ok := providers.Get(providerConfig.Spec.Type)
	if !ok {
		return nil, fmt.Errorf("provider type %s not supported", providerConfig.Spec.Type)
	}

	// Initialize provider with the appropriate configuration
	options := make(map[string]interface{})
	options["name"] = providerConfig.Name

	// Add type-specific configuration
	switch providerConfig.Spec.Type {
	case "github":
		if providerConfig.Spec.GitHub != nil {
			options["enterprise"] = providerConfig.Spec.GitHub.Enterprise
			if providerConfig.Spec.GitHub.PollingInterval != "" {
				options["cacheTTL"] = providerConfig.Spec.GitHub.PollingInterval
			}

			// Get API token from secret if specified
			if providerConfig.Spec.GitHub.API.SecretRef.Name != "" {
				apiToken, err := r.SecretReader.GetSecret(
					ctx,
					providerConfig.Spec.GitHub.API.SecretRef.Namespace,
					providerConfig.Spec.GitHub.API.SecretRef.Name,
					providerConfig.Spec.GitHub.API.SecretRef.Key,
				)
				if err != nil {
					return nil, fmt.Errorf("error reading GitHub API token: %w", err)
				}
				options["apiToken"] = apiToken
			}
		}
	case "aws":
		if providerConfig.Spec.AWS != nil {
			if len(providerConfig.Spec.AWS.Services) > 0 {
				options["services"] = providerConfig.Spec.AWS.Services
			}
			if len(providerConfig.Spec.AWS.Regions) > 0 {
				options["regions"] = providerConfig.Spec.AWS.Regions
			}
			if providerConfig.Spec.AWS.PollingInterval != "" {
				options["cacheTTL"] = providerConfig.Spec.AWS.PollingInterval
			}

			// Get API credentials from secret if specified
			if providerConfig.Spec.AWS.API.SecretRef.Name != "" {
				accessKey, err := r.SecretReader.GetSecret(
					ctx,
					providerConfig.Spec.AWS.API.SecretRef.Namespace,
					providerConfig.Spec.AWS.API.SecretRef.Name,
					"accessKey",
				)
				if err != nil {
					return nil, fmt.Errorf("error reading AWS access key: %w", err)
				}
				options["accessKey"] = accessKey

				secretKey, err := r.SecretReader.GetSecret(
					ctx,
					providerConfig.Spec.AWS.API.SecretRef.Namespace,
					providerConfig.Spec.AWS.API.SecretRef.Name,
					"secretKey",
				)
				if err != nil {
					return nil, fmt.Errorf("error reading AWS secret key: %w", err)
				}
				options["secretKey"] = secretKey
			}
		}
	}

	// Initialize the provider
	if err := providerInstance.Init(ctx, options); err != nil {
		return nil, fmt.Errorf("error initializing provider: %w", err)
	}

	// Cache the provider for future use
	r.ProviderCache[providerConfig.Name] = providerInstance

	return providerInstance, nil
}

// applyIPRangesToIngress applies IP ranges to all ingress services
func (r *SyncReconciler) applyIPRangesToIngress(ctx context.Context, syncConfig *ingressmetasyncv1alpha1.SyncConfig, ipRanges *model.IPRangeSet) error {
	updatedIngressStatus := make([]ingressmetasyncv1alpha1.IngressSyncStatus, 0)

	for _, ingressRef := range syncConfig.Spec.Ingress {
		ingressStatus := ingressmetasyncv1alpha1.IngressSyncStatus{
			Name:         ingressRef.Name,
			LastSyncTime: &metav1.Time{Time: time.Now()},
			Status:       "Pending",
		}

		// Fetch the IngressConfig
		var ingressConfig ingressmetasyncv1alpha1.IngressConfig
		if err := r.Get(ctx, types.NamespacedName{Name: ingressRef.Name}, &ingressConfig); err != nil {
			errMsg := fmt.Sprintf("Unable to fetch IngressConfig: %v", err)
			ingressStatus.Error = errMsg
			ingressStatus.Status = "Error"
			updatedIngressStatus = append(updatedIngressStatus, ingressStatus)
			
			if syncConfig.Spec.SyncPolicy != nil && syncConfig.Spec.SyncPolicy.FailureMode == "fail" {
				return fmt.Errorf(errMsg)
			}
			
			r.Log.Error(err, "Unable to fetch IngressConfig", "ingress", ingressRef.Name)
			continue
		}

		// Get the ingress instance from cache or create a new one
		ingressInstance, err := r.getOrCreateIngress(ctx, &ingressConfig)
		if err != nil {
			errMsg := fmt.Sprintf("Unable to create ingress: %v", err)
			ingressStatus.Error = errMsg
			ingressStatus.Status = "Error"
			updatedIngressStatus = append(updatedIngressStatus, ingressStatus)
			
			if syncConfig.Spec.SyncPolicy != nil && syncConfig.Spec.SyncPolicy.FailureMode == "fail" {
				return fmt.Errorf(errMsg)
			}
			
			r.Log.Error(err, "Unable to create ingress", "ingress", ingressRef.Name)
			continue
		}

		// Apply IP ranges to the ingress
		if err := ingressInstance.ApplyIPRanges(ctx, ipRanges); err != nil {
			errMsg := fmt.Sprintf("Unable to apply IP ranges: %v", err)
			ingressStatus.Error = errMsg
			ingressStatus.Status = "Error"
			updatedIngressStatus = append(updatedIngressStatus, ingressStatus)
			
			if syncConfig.Spec.SyncPolicy != nil && syncConfig.Spec.SyncPolicy.FailureMode == "fail" {
				return fmt.Errorf(errMsg)
			}
			
			r.Log.Error(err, "Unable to apply IP ranges", "ingress", ingressRef.Name)
			continue
		}

		// Update ingress status
		ingressStatus.Status = "Success"
		ingressStatus.IPRangesCount = int32(ipRanges.Count())
		updatedIngressStatus = append(updatedIngressStatus, ingressStatus)
	}

	// Update the SyncConfig status with ingress status
	syncConfig.Status.IngressStatus = updatedIngressStatus

	return nil
}

// getOrCreateIngress gets an existing ingress from cache or creates a new one
func (r *SyncReconciler) getOrCreateIngress(ctx context.Context, ingressConfig *ingressmetasyncv1alpha1.IngressConfig) (ingress.Ingress, error) {
	// Check if we already have this ingress in the cache
	if ingressInstance, ok := r.IngressCache[ingressConfig.Name]; ok {
		return ingressInstance, nil
	}

	// Create a new ingress instance
	ingressInstance, ok := ingress.Get(ingressConfig.Spec.Type)
	if !ok {
		return nil, fmt.Errorf("ingress type %s not supported", ingressConfig.Spec.Type)
	}

	// Initialize ingress with the appropriate configuration
	options := make(map[string]interface{})
	options["name"] = ingressConfig.Name

	// Add type-specific configuration
	switch ingressConfig.Spec.Type {
	case "cloudflare":
		if ingressConfig.Spec.Cloudflare != nil {
			// Get API token from secret
			if ingressConfig.Spec.Cloudflare.API.SecretRef.Name != "" {
				apiToken, err := r.SecretReader.GetSecret(
					ctx,
					ingressConfig.Spec.Cloudflare.API.SecretRef.Namespace,
					ingressConfig.Spec.Cloudflare.API.SecretRef.Name,
					ingressConfig.Spec.Cloudflare.API.SecretRef.Key,
				)
				if err != nil {
					return nil, fmt.Errorf("error reading Cloudflare API token: %w", err)
				}
				options["apiToken"] = apiToken
			}

			// Add rule configuration
			options["zoneId"] = ingressConfig.Spec.Cloudflare.RuleConfig.ZoneID
			options["ruleName"] = ingressConfig.Spec.Cloudflare.RuleConfig.RuleName
			
			if ingressConfig.Spec.Cloudflare.RuleConfig.Description != "" {
				options["description"] = ingressConfig.Spec.Cloudflare.RuleConfig.Description
			}
			
			if ingressConfig.Spec.Cloudflare.RuleConfig.Action != "" {
				options["action"] = ingressConfig.Spec.Cloudflare.RuleConfig.Action
			}
			
			if ingressConfig.Spec.Cloudflare.RuleConfig.Priority != 0 {
				options["priority"] = ingressConfig.Spec.Cloudflare.RuleConfig.Priority
			}
			
			if ingressConfig.Spec.Cloudflare.UpdateStrategy != "" {
				options["updateStrategy"] = ingressConfig.Spec.Cloudflare.UpdateStrategy
			}
		}
	case "istio":
		if ingressConfig.Spec.Istio != nil {
			if ingressConfig.Spec.Istio.Namespace != "" {
				options["namespace"] = ingressConfig.Spec.Istio.Namespace
			}

			// Add X-Forwarded-For configuration
			options["xForwardedForConfig"] = map[string]interface{}{
				"enabled":    ingressConfig.Spec.Istio.XForwardedForConfig.Enabled,
				"headerName": ingressConfig.Spec.Istio.XForwardedForConfig.HeaderName,
			}

			// Add gateway selector
			options["gatewaySelector"] = map[string]interface{}{
				"name":      ingressConfig.Spec.Istio.GatewaySelector.Name,
				"namespace": ingressConfig.Spec.Istio.GatewaySelector.Namespace,
				"labels":    ingressConfig.Spec.Istio.GatewaySelector.Labels,
			}
		}
	}

	// Initialize the ingress
	if err := ingressInstance.Init(ctx, options); err != nil {
		return nil, fmt.Errorf("error initializing ingress: %w", err)
	}

	// Cache the ingress for future use
	r.IngressCache[ingressConfig.Name] = ingressInstance

	return ingressInstance, nil
}
