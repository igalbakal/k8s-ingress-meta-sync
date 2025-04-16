package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressConfigSpec defines the desired state of IngressConfig
type IngressConfigSpec struct {
	// Type specifies the ingress type (cloudflare, istio, etc.)
	// +kubebuilder:validation:Enum=cloudflare;istio
	Type string `json:"type"`
	
	// Cloudflare specific configuration
	// +optional
	Cloudflare *CloudflareIngressConfig `json:"cloudflare,omitempty"`
	
	// Istio specific configuration
	// +optional
	Istio *IstioIngressConfig `json:"istio,omitempty"`
}

// CloudflareIngressConfig contains Cloudflare specific configuration
type CloudflareIngressConfig struct {
	// API configuration
	API CloudflareAPIConfig `json:"api"`
	
	// RuleConfig defines the Cloudflare rule configuration
	RuleConfig CloudflareRuleConfig `json:"ruleConfig"`
	
	// UpdateStrategy defines how to apply updates
	// +optional
	// +kubebuilder:default="direct"
	// +kubebuilder:validation:Enum=direct;incremental
	UpdateStrategy string `json:"updateStrategy,omitempty"`
}

// CloudflareAPIConfig contains configuration for Cloudflare API
type CloudflareAPIConfig struct {
	// SecretRef points to a Kubernetes Secret containing API credentials
	SecretRef SecretReference `json:"secretRef"`
}

// CloudflareRuleConfig contains configuration for Cloudflare rule
type CloudflareRuleConfig struct {
	// ZoneID is the Cloudflare zone ID
	ZoneID string `json:"zoneId"`
	
	// RuleName is the name of the Cloudflare rule
	RuleName string `json:"ruleName"`
	
	// Description is a description for the rule
	// +optional
	Description string `json:"description,omitempty"`
	
	// Action for the rule (e.g., "allow", "block", "challenge")
	// +optional
	// +kubebuilder:default="allow"
	Action string `json:"action,omitempty"`
	
	// Priority defines the rule's priority
	// +optional
	Priority int32 `json:"priority,omitempty"`
}

// IstioIngressConfig contains Istio specific configuration
type IstioIngressConfig struct {
	// Namespace is the namespace for Istio resources
	// +optional
	// +kubebuilder:default="istio-system"
	Namespace string `json:"namespace,omitempty"`
	
	// XForwardedForConfig configures header enrichment for client IP tracking
	XForwardedForConfig XForwardedForConfig `json:"xForwardedForConfig"`
	
	// GatewaySelector selects which Istio gateway to configure
	GatewaySelector GatewaySelector `json:"gatewaySelector"`
}

// XForwardedForConfig configures x-forwarded-for header handling
type XForwardedForConfig struct {
	// Enabled indicates whether x-forwarded-for header enrichment is enabled
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`
	
	// HeaderName is the name of the header
	// +optional
	// +kubebuilder:default="X-Forwarded-For"
	HeaderName string `json:"headerName,omitempty"`
}

// GatewaySelector selects which Istio gateway to configure
type GatewaySelector struct {
	// Name of the gateway
	// +optional
	// +kubebuilder:default="ingressgateway"
	Name string `json:"name,omitempty"`
	
	// Namespace of the gateway
	// +optional
	Namespace string `json:"namespace,omitempty"`
	
	// Labels to select gateway by label
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// IngressConfigStatus defines the observed state of IngressConfig
type IngressConfigStatus struct {
	// LastSyncTime is the last time the ingress was synced
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	
	// LastSuccessfulSync is the last time the ingress was successfully synced
	// +optional
	LastSuccessfulSync *metav1.Time `json:"lastSuccessfulSync,omitempty"`
	
	// IPRangesCount is the number of IP ranges currently configured
	// +optional
	IPRangesCount int32 `json:"ipRangesCount,omitempty"`
	
	// LastSyncError is the last error encountered during sync
	// +optional
	LastSyncError string `json:"lastSyncError,omitempty"`
	
	// Conditions represent the latest available observations of the Ingress's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="Last Sync",type="date",JSONPath=".status.lastSuccessfulSync"
//+kubebuilder:printcolumn:name="IP Ranges",type="integer",JSONPath=".status.ipRangesCount"

// IngressConfig is the Schema for the ingressconfigs API
type IngressConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressConfigSpec   `json:"spec,omitempty"`
	Status IngressConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IngressConfigList contains a list of IngressConfig
type IngressConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IngressConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IngressConfig{}, &IngressConfigList{})
}
