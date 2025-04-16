package providers

import (
	"context"
	
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/model"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Provider defines the interface for fetching IP ranges from different sources
type Provider interface {
	// Name returns the name of the provider
	Name() string
	
	// Type returns the type of the provider (e.g., "github", "aws")
	Type() string
	
	// Init initializes the provider with the given options
	Init(ctx context.Context, options map[string]interface{}) error
	
	// FetchIPRanges fetches the IP ranges from the provider
	FetchIPRanges(ctx context.Context) (*model.IPRangeSet, error)
}

var log = ctrl.Log.WithName("providers")

// Registry is a registry of available providers
type Registry struct {
	providers map[string]func() Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]func() Provider),
	}
}

// Register registers a provider factory
func (r *Registry) Register(providerType string, factory func() Provider) {
	r.providers[providerType] = factory
}

// Get returns a provider for the given type
func (r *Registry) Get(providerType string) (Provider, bool) {
	factory, exists := r.providers[providerType]
	if !exists {
		return nil, false
	}
	return factory(), true
}

// DefaultRegistry is the default provider registry
var DefaultRegistry = NewRegistry()

// Get returns a provider from the default registry
func Get(providerType string) (Provider, bool) {
	return DefaultRegistry.Get(providerType)
}

// Register registers a provider with the default registry
func Register(providerType string, factory func() Provider) {
	DefaultRegistry.Register(providerType, factory)
}
