package ingress

import (
	"context"
	
	"github.com/galbakal/k8s-ingress-meta-sync/pkg/model"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Ingress defines the interface for applying IP ranges to ingress services
type Ingress interface {
	// Name returns the name of the ingress
	Name() string
	
	// Type returns the type of the ingress (e.g., "cloudflare", "istio")
	Type() string
	
	// Init initializes the ingress with the given options
	Init(ctx context.Context, options map[string]interface{}) error
	
	// ApplyIPRanges applies the given IP ranges to the ingress
	ApplyIPRanges(ctx context.Context, ipRanges *model.IPRangeSet) error
	
	// GetCurrentIPRanges gets the current IP ranges configured in the ingress
	GetCurrentIPRanges(ctx context.Context) (*model.IPRangeSet, error)
}

var log = ctrl.Log.WithName("ingress")

// Registry is a registry of available ingress services
type Registry struct {
	ingressServices map[string]func() Ingress
}

// NewRegistry creates a new ingress registry
func NewRegistry() *Registry {
	return &Registry{
		ingressServices: make(map[string]func() Ingress),
	}
}

// Register registers an ingress factory
func (r *Registry) Register(ingressType string, factory func() Ingress) {
	r.ingressServices[ingressType] = factory
}

// Get returns an ingress for the given type
func (r *Registry) Get(ingressType string) (Ingress, bool) {
	factory, exists := r.ingressServices[ingressType]
	if !exists {
		return nil, false
	}
	return factory(), true
}

// DefaultRegistry is the default ingress registry
var DefaultRegistry = NewRegistry()

// Get returns an ingress from the default registry
func Get(ingressType string) (Ingress, bool) {
	return DefaultRegistry.Get(ingressType)
}

// Register registers an ingress with the default registry
func Register(ingressType string, factory func() Ingress) {
	DefaultRegistry.Register(ingressType, factory)
}
