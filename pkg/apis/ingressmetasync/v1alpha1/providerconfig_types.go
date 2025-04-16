package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderConfigSpec defines the desired state of ProviderConfig
type ProviderConfigSpec struct {
	// Type specifies the provider type (github, aws, etc.)
	// +kubebuilder:validation:Enum=github;aws
	Type string `json:"type"`
	
	// GitHub specific configuration
	// +optional
	GitHub *GitHubProviderConfig `json:"github,omitempty"`
	
	// AWS specific configuration
	// +optional
	AWS *AWSProviderConfig `json:"aws,omitempty"`
}

// GitHubProviderConfig contains GitHub specific configuration
type GitHubProviderConfig struct {
	// Enterprise toggles between Enterprise and public GitHub
	// +optional
	// +kubebuilder:default=true
	Enterprise bool `json:"enterprise,omitempty"`
	
	// API configuration
	API GitHubAPIConfig `json:"api"`
	
	// PollingInterval defines how often to check for IP range updates
	// +optional
	// +kubebuilder:default="1m"
	PollingInterval string `json:"pollingInterval,omitempty"`
}

// GitHubAPIConfig contains configuration for GitHub API
type GitHubAPIConfig struct {
	// SecretRef points to a Kubernetes Secret containing API credentials
	SecretRef SecretReference `json:"secretRef"`
}

// AWSProviderConfig contains AWS specific configuration
type AWSProviderConfig struct {
	// API configuration
	API AWSAPIConfig `json:"api"`
	
	// Services defines which AWS service IP ranges to include
	// +optional
	// +kubebuilder:default={"AMAZON"}
	Services []string `json:"services,omitempty"`
	
	// Regions defines which AWS regions to include
	// +optional
	Regions []string `json:"regions,omitempty"`
	
	// PollingInterval defines how often to check for IP range updates
	// +optional
	// +kubebuilder:default="1m"
	PollingInterval string `json:"pollingInterval,omitempty"`
}

// AWSAPIConfig contains configuration for AWS API
type AWSAPIConfig struct {
	// SecretRef points to a Kubernetes Secret containing API credentials
	SecretRef SecretReference `json:"secretRef"`
}

// SecretReference contains information that points to the Kubernetes Secret being used
type SecretReference struct {
	// Name is the name of the secret
	Name string `json:"name"`
	
	// Namespace is the namespace of the secret
	Namespace string `json:"namespace"`
	
	// Key is the specific key in the secret that contains the required data
	// +optional
	Key string `json:"key,omitempty"`
}

// ProviderConfigStatus defines the observed state of ProviderConfig
type ProviderConfigStatus struct {
	// LastSyncTime is the last time the provider successfully synced IP metadata
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	
	// LastSuccessfulSync is the last time the provider successfully synced IP metadata
	// +optional
	LastSuccessfulSync *metav1.Time `json:"lastSuccessfulSync,omitempty"`
	
	// FailedAttempts is the number of sequential failed attempts
	// +optional
	FailedAttempts int32 `json:"failedAttempts,omitempty"`
	
	// Conditions represent the latest available observations of the Provider's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="Last Sync",type="date",JSONPath=".status.lastSuccessfulSync"

// ProviderConfig is the Schema for the providerconfigs API
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec,omitempty"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}
