# KubeConfig Conversion Example

This example demonstrates how to use the kubeconfig conversion feature to convert a Cluster API (CAPI) kubeconfig Secret to a Flux-compatible ConfigMap for use with workload identity.

## Prerequisites

- A Kubernetes cluster with Cluster API installed
- Flux Operator installed
- A CAPI-managed cluster with a kubeconfig Secret

## Example CAPI KubeConfig Secret

Cluster API creates a Secret like this when provisioning a cluster:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: capi-helloworld-kubeconfig
  namespace: default
  labels:
    cluster.x-k8s.io/cluster-name: capi-helloworld
type: cluster.x-k8s.io/secret
stringData:
  value: |
    apiVersion: v1
    clusters:
    - cluster:
        certificate-authority-data: LS0tLS1CRUdJTi...
        server: https://172.18.0.3:6443
      name: capi-helloworld
    contexts:
    - context:
        cluster: capi-helloworld
        user: capi-helloworld-admin
      name: capi-helloworld-admin@capi-helloworld
    current-context: capi-helloworld-admin@capi-helloworld
    kind: Config
    preferences: {}
    users:
    - name: capi-helloworld-admin
      user:
        client-certificate-data: LS0tLS1...  # Long-lived credential
        client-key-data: LS0tLS1...          # Long-lived credential
```

## ResourceSet with KubeConfig Conversion

Create a ResourceSet that converts the CAPI Secret to a Flux ConfigMap:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: workload-cluster-config
  namespace: flux-system
spec:
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: remote-cluster-config
        namespace: flux-system
        annotations:
          # This annotation triggers the conversion
          fluxcd.controlplane.io/convertKubeConfigFrom: "default/capi-helloworld-kubeconfig"
      data:
        # These fields are user-provided and will be preserved
        provider: "aws"
        serviceAccountName: "flux-reconciler"
        audiences: "sts.amazonaws.com"
        # The 'server' and 'ca.crt' fields will be automatically extracted
        # from the CAPI kubeconfig Secret and merged here
```

## Resulting ConfigMap

After reconciliation, the ResourceSet controller will create this ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: remote-cluster-config
  namespace: flux-system
data:
  # User-provided fields (from template)
  provider: "aws"
  serviceAccountName: "flux-reconciler"
  audiences: "sts.amazonaws.com"
  
  # Automatically extracted fields (from CAPI Secret)
  server: "https://172.18.0.3:6443"
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    MIICxxxxxxx...
    -----END CERTIFICATE-----
```

## Using with Flux Kustomization

Now you can use this ConfigMap with Flux's workload identity feature:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: apps-on-remote-cluster
  namespace: flux-system
spec:
  interval: 10m
  path: ./apps
  prune: true
  sourceRef:
    kind: GitRepository
    name: my-repo
  kubeConfig:
    # Use the converted ConfigMap instead of the Secret
    configMapRef:
      name: remote-cluster-config
```

## Security Benefits

**Before (using Secret with long-lived credentials):**
- ❌ Cluster-admin kubeconfig with client certificates stored in multiple namespaces
- ❌ Long-lived credentials increase attack surface
- ❌ Credentials can be extracted and used outside the cluster

**After (using ConfigMap with workload identity):**
- ✅ Only API server address and CA certificate in ConfigMap (no credentials)
- ✅ Short-lived credentials via workload identity (IRSA, Workload Identity, etc.)
- ✅ Cluster-admin kubeconfig Secret stays in single namespace
- ✅ Better multi-tenancy security posture

## Multi-Tenant Example

For multi-tenant environments, you can keep the CAPI kubeconfig Secret in a restricted namespace:

```yaml
---
# CAPI kubeconfig Secret (restricted namespace)
apiVersion: v1
kind: Secret
metadata:
  name: prod-cluster-kubeconfig
  namespace: cluster-admin  # Only admins have access
type: cluster.x-k8s.io/secret
stringData:
  value: |
    # Full kubeconfig with credentials

---
# ResourceSet in flux-system (admin namespace)
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: prod-cluster-configs
  namespace: flux-system
spec:
  resources:
    # Create ConfigMaps for different tenants
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: prod-cluster-config
        namespace: tenant-a
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "cluster-admin/prod-cluster-kubeconfig"
      data:
        provider: "aws"
        serviceAccountName: "tenant-a-reconciler"
        audiences: "sts.amazonaws.com"
    
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: prod-cluster-config
        namespace: tenant-b
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "cluster-admin/prod-cluster-kubeconfig"
      data:
        provider: "aws"
        serviceAccountName: "tenant-b-reconciler"
        audiences: "sts.amazonaws.com"
```

Now:
- Tenants get ConfigMaps with only server address and CA
- Each tenant uses their own service account with limited permissions
- The cluster-admin kubeconfig Secret stays in the `cluster-admin` namespace
- Tenants can't access the long-lived credentials

## Troubleshooting

### ConfigMap not created

Check the ResourceSet status:
```bash
kubectl describe resourceset workload-cluster-config -n flux-system
```

Look for events related to kubeconfig conversion.

### Permission denied

The Flux Operator needs RBAC permissions to read Secrets:
```bash
kubectl auth can-i get secrets --as=system:serviceaccount:flux-system:flux-operator -n default
```

### Invalid kubeconfig format

The conversion expects CAPI kubeconfig Secrets with a `value` field containing the kubeconfig YAML. Verify:
```bash
kubectl get secret capi-helloworld-kubeconfig -n default -o jsonpath='{.data.value}' | base64 -d
```

## References

- [Flux Workload Identity Documentation](https://fluxcd.io/flux/components/kustomize/kustomizations/#kubeconfig-reference)
- [Cluster API Documentation](https://cluster-api.sigs.k8s.io/)
- [ResourceSet API Documentation](../api/v1/resourceset.md)
