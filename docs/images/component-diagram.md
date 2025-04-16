graph TD
    subgraph "Kubernetes Cluster"
        subgraph "Controller"
            A[Reconciliation Loop] --> B[Provider Manager]
            A --> C[Ingress Manager]
            A --> D[IP Range Differ]
            
            B --> B1[Provider Registry]
            B1 --> B1_1[GitHub Provider]
            B1 --> B1_2[AWS Provider]
            
            C --> C1[Ingress Registry]
            C1 --> C1_1[Cloudflare Ingress]
            C1 --> C1_2[Istio Ingress]
            
            D --> D1[IPRangeSet]
            D1 --> D1_1[Add/Remove Detection]
        end
        
        subgraph "API Objects"
            E[ProviderConfig]
            F[IngressConfig]
            G[SyncConfig]
            
            G --> E
            G --> F
        end
        
        A --> G
        B --> E
        C --> F
    end
    
    subgraph "External Systems"
        H[GitHub API] <--> B1_1
        I[AWS IP Service] <--> B1_2
        J[Cloudflare API] <--> C1_1
        K[Istio Gateway API] <--> C1_2
    end
    
    style B1_2 stroke-dasharray: 5 5
    style I stroke-dasharray: 5 5
    
    classDef controller fill:#e1f5fe,stroke:#01579b,stroke-width:2px;
    classDef apiobjects fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px;
    classDef external fill:#ffebee,stroke:#b71c1c,stroke-width:2px;
    
    class A,B,C,D,B1,B1_1,B1_2,C1,C1_1,C1_2,D1,D1_1 controller;
    class E,F,G apiobjects;
    class H,I,J,K external;
```

This diagram details the component architecture of the K8s-Ingress-Meta-Sync controller:

1. **Reconciliation Loop**: The core workflow that processes SyncConfig resources:
   - Fetches IP ranges from configured providers
   - Applies IP ranges to configured ingress services
   - Detects and reacts to changes

2. **Provider Manager**: Manages provider instances and their lifecycle:
   - Provider Registry: Maintains available provider types
   - Provider Implementations: GitHub (implemented), AWS (future)

3. **Ingress Manager**: Manages ingress service instances and their lifecycle:
   - Ingress Registry: Maintains available ingress types
   - Ingress Implementations: Cloudflare and Istio

4. **IP Range Differ**: Compares IP range sets to identify changes:
   - Identifies added and removed IP ranges
   - Filters IP ranges based on inclusion/exclusion rules

5. **API Objects**: Kubernetes custom resources that define the configuration:
   - ProviderConfig: Configures IP range sources
   - IngressConfig: Configures ingress services
   - SyncConfig: Maps providers to ingress services with sync policies

The controller interacts with external systems to fetch IP ranges and apply them to ingress services.
