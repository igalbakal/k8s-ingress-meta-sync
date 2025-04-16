graph TD
    subgraph "K8s Cluster"
        A[K8s-Ingress-Meta-Sync Controller] --> B[Provider Subsystem]
        A --> C[Ingress Subsystem]
        A --> D[Reconciliation Engine]
        
        B --> B1[GitHub Provider]
        B --> B2[AWS Provider]
        B --> B3[Provider Interface]
        
        C --> C1[Cloudflare Ingress]
        C --> C2[Istio Ingress]
        C --> C3[Ingress Interface]
        
        D --> D1[Controller Logic]
        D --> D2[IP Range Differ]
        D --> D3[Event Logger]

        E[Custom Resources] --> E1[ProviderConfig CRD]
        E --> E2[IngressConfig CRD]
        E --> E3[SyncConfig CRD]
    end

    F[GitHub API] <--> B1
    G[AWS IP Service] <--> B2
    H[Cloudflare API] <--> C1
    I[Istio Gateway] <--> C2

    style B2 stroke-dasharray: 5 5
    style G stroke-dasharray: 5 5

    classDef external fill:#f9f,stroke:#333,stroke-width:2px;
    class F,G,H,I external;
```

This diagram shows the overall architecture of the K8s-Ingress-Meta-Sync system. The controller has three main subsystems:

1. **Provider Subsystem**: Responsible for fetching IP ranges from different sources
   - Implemented providers: GitHub
   - Future providers: AWS (shown with dashed line)

2. **Ingress Subsystem**: Responsible for applying IP ranges to different ingress services
   - Implemented ingress types: Cloudflare, Istio

3. **Reconciliation Engine**: Manages the synchronization process
   - Handles diffs between current and desired state
   - Manages error handling and retries
   - Records events for auditing

The system uses Custom Resource Definitions (CRDs) to configure providers, ingress services, and sync mappings between them.
