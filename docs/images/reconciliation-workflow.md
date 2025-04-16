sequenceDiagram
    participant C as Controller
    participant P as Provider (GitHub)
    participant D as Differ
    participant I as Ingress (Cloudflare/Istio)
    participant K as Kubernetes API
    
    C->>K: Watch SyncConfig changes
    activate C
    
    Note over C: Reconciliation begins
    
    C->>K: Get SyncConfig
    K-->>C: SyncConfig
    
    loop For each provider in SyncConfig
        C->>K: Get ProviderConfig
        K-->>C: ProviderConfig
        
        C->>P: Init provider
        P-->>C: Provider instance
        
        C->>P: Fetch IP ranges
        P-->>C: IP ranges
        
        Note over C,P: Apply include/exclude filters
    end
    
    Note over C: Merge IP ranges from all providers
    
    loop For each ingress in SyncConfig
        C->>K: Get IngressConfig
        K-->>C: IngressConfig
        
        C->>I: Init ingress
        I-->>C: Ingress instance
        
        C->>I: Get current IP ranges
        I-->>C: Current IP ranges
        
        C->>D: Compare IP ranges
        D-->>C: Added/removed ranges
        
        alt IP ranges changed
            C->>I: Apply IP ranges
            I-->>C: Update status
        end
    end
    
    C->>K: Update SyncConfig status
    
    Note over C: Schedule next reconciliation
    
    deactivate C
```

This sequence diagram illustrates the detailed workflow of the reconciliation process:

1. **Initialization**: The controller watches for changes to SyncConfig resources
   
2. **Provider Processing**:
   - For each provider configured in the SyncConfig:
     - Fetch the associated ProviderConfig
     - Initialize the provider with appropriate configuration
     - Fetch IP ranges from the provider
     - Apply any include/exclude filters specified in the SyncConfig
   
3. **IP Range Merging**:
   - Combine IP ranges from all providers into a single set
   
4. **Ingress Processing**:
   - For each ingress configured in the SyncConfig:
     - Fetch the associated IngressConfig
     - Initialize the ingress with appropriate configuration
     - Get the current IP ranges configured in the ingress
     - Compare with the new IP ranges to detect changes
     - If changes are detected, apply the new IP ranges to the ingress
   
5. **Status Update**:
   - Update the SyncConfig status with results from each provider and ingress
   - Record any errors or warnings
   
6. **Scheduling**:
   - Schedule the next reconciliation based on polling intervals

This workflow ensures that IP ranges are continuously monitored and synchronized, with proper error handling and status reporting.
