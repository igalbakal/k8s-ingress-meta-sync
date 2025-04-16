package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncConfigSpec defines the desired state of SyncConfig
type SyncConfigSpec struct {
	// Providers is a list of provider configurations to sync from
	Providers []ProviderReference `json:"providers"`
	
	// Ingress is a list of ingress configurations to sync to
	Ingress []IngressReference `json:"ingress"`
	
	// SyncPolicy defines how syncing should be handled
	// +optional
	SyncPolicy *SyncPolicy `json:"syncPolicy,omitempty"`
}

// ProviderReference references a ProviderConfig
type ProviderReference struct {
	// Name of the ProviderConfig
	Name string `json:"name"`
	
	// IncludeRanges specifies which IP ranges to include
	// For GitHub, this could be "web", "api", "git", etc.
	// For AWS, this could be service names
	// +optional
	IncludeRanges []string `json:"includeRanges,omitempty"`
	
	// ExcludeRanges specifies which IP ranges to exclude
	// +optional
	ExcludeRanges []string `json:"excludeRanges,omitempty"`
}

// IngressReference references an IngressConfig
type IngressReference struct {
	// Name of the IngressConfig
	Name string `json:"name"`
}

// SyncPolicy defines how to handle sync failures and retries
type SyncPolicy struct {
	// FailureMode defines what to do when a sync fails
	// +optional
	// +kubebuilder:default="continue"
	// +kubebuilder:validation:Enum=continue;fail
	FailureMode string `json:"failureMode,omitempty"`
	
	// RetryConfig defines retry behavior
	// +optional
	RetryConfig *RetryConfig `json:"retryConfig,omitempty"`
}

// RetryConfig defines retry behavior for failed syncs
type RetryConfig struct {
	// MaxRetries is the maximum number of retries
	// +optional
	// +kubebuilder:default=3
	MaxRetries int32 `json:"maxRetries,omitempty"`
	
	// BackoffMultiplier is the multiplier for exponential backoff
	// +optional
	// +kubebuilder:default=2
	BackoffMultiplier int32 `json:"backoffMultiplier,omitempty"`
	
	// InitialDelaySeconds is the initial delay before first retry
	// +optional
	// +kubebuilder:default=5
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
}

// SyncConfigStatus defines the observed state of SyncConfig
type SyncConfigStatus struct {
	// LastSyncTime is the last time a sync was attempted
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	
	// LastSuccessfulSync is the last time a sync was successful
	// +optional
	LastSuccessfulSync *metav1.Time `json:"lastSuccessfulSync,omitempty"`
	
	// ProviderStatus contains sync status for each provider
	// +optional
	ProviderStatus []ProviderSyncStatus `json:"providerStatus,omitempty"`
	
	// IngressStatus contains sync status for each ingress
	// +optional
	IngressStatus []IngressSyncStatus `json:"ingressStatus,omitempty"`
	
	// Conditions represent the latest available observations of SyncConfig's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ProviderSyncStatus represents the sync status for a specific provider
type ProviderSyncStatus struct {
	// Name of the provider
	Name string `json:"name"`
	
	// LastSyncTime is the last time this provider was synced
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	
	// Status of the sync
	// +optional
	Status string `json:"status,omitempty"`
	
	// IPRangesCount is the count of IP ranges from this provider
	// +optional
	IPRangesCount int32 `json:"ipRangesCount,omitempty"`
	
	// Error is the last error encountered with this provider
	// +optional
	Error string `json:"error,omitempty"`
}

// IngressSyncStatus represents the sync status for a specific ingress
type IngressSyncStatus struct {
	// Name of the ingress
	Name string `json:"name"`
	
	// LastSyncTime is the last time this ingress was synced
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	
	// Status of the sync
	// +optional
	Status string `json:"status,omitempty"`
	
	// IPRangesCount is the count of IP ranges synced to this ingress
	// +optional
	IPRangesCount int32 `json:"ipRangesCount,omitempty"`
	
	// Error is the last error encountered with this ingress
	// +optional
	Error string `json:"error,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="Last Sync",type="date",JSONPath=".status.lastSuccessfulSync"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"

// SyncConfig is the Schema for the syncconfigs API
type SyncConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncConfigSpec   `json:"spec,omitempty"`
	Status SyncConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SyncConfigList contains a list of SyncConfig
type SyncConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyncConfig{}, &SyncConfigList{})
}
