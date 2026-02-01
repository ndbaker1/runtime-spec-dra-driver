# OCI RuntimeSpec DRA Driver

An experimental DRA (Dynamic Resource Allocation) driver that allows setting arbitrary 
OCI runtime spec fields on containers via configuration associated with a `ResourceClaim`.

## Goals

- **Runtime Spec Modification via DRA**: Expose OCI runtime spec fields through Kubernetes 
  Dynamic Resource Allocation, allowing workloads to request specific runtime configurations 
  via ResourceClaims.

- **cgroupv2 Unified Parameter Support**: Enable direct control of cgroup v2 unified 
  parameters (e.g., `io.max` for I/O throttling, `memory.high` for memory limits) that 
  must be set before container start.

- **NRI Integration**: Use [Node Resource Interface (NRI)](https://github.com/containerd/nri) 
  to modify container specs during the `CreateContainer` phase, ensuring cgroup changes 
  take effect before the container starts.

### Data Flow

1. User creates `ResourceClaim` with `RuntimeSpecEditConfig` containing OCI spec fields
2. **DRA Plugin** (`PrepareResourceClaims`):
   - Parses the `RuntimeSpecEditConfig` from the claim
   - Encodes the spec as `OCI_RUNTIME_SPEC` environment variable via CDI
3. **containerd/CRI-O** applies CDI container edits (including env var)
4. **NRI Plugin** (on `CreateContainer` event):
   - Reads `OCI_RUNTIME_SPEC` from container environment
   - Parses OCI spec and creates container adjustments
   - Returns adjustment to runtime (unified cgroup params, mounts, env, etc.)
5. Container starts with correct cgroup configuration

## Unified Cgroup Parameters

The `unified` field in the OCI runtime spec allows setting arbitrary cgroup v2 parameters.
Each key maps to a file in the cgroup unified hierarchy. See the 
[OCI runtime spec](https://github.com/opencontainers/runtime-spec/blob/main/config-linux.md#unified)
for details.

### Example: I/O Throttling

```yaml
apiVersion: resource.k8s.io/v1beta1
kind: ResourceClaimTemplate
metadata:
  name: io-throttle
spec:
  spec:
    devices:
      requests:
      - name: io-config
        deviceClassName: runtime-spec.io
      config:
      - requests: [io-config]
        opaque:
          driver: runtime-spec.io
          parameters:
            apiVersion: dra.runtime-spec.io/v1alpha1
            kind: RuntimeSpecEditConfig
            spec:
              linux:
                resources:
                  unified:
                    # Throttle device 259:0 to 2MB/s read, 120 write IOPS
                    "io.max": "259:0 rbps=2097152 wiops=120"
```

### Example: Multiple Parameters

```yaml
spec:
  linux:
    resources:
      unified:
        "io.max": "259:0 rbps=2097152"
        "memory.high": "268435456"
        "pids.max": "100"
```

## Prerequisites

- Kubernetes 1.32+ with DRA feature gate enabled
- containerd v1.7.0+ or CRI-O v1.26.0+ with NRI enabled
- cgroupv2 on nodes

## Quickstart

```bash
export REGISTRY=<your-registry>
export VERSION=v0.1.0
export IMAGE_NAME=runtime-spec-dra-driver

# Build and push images
make generate
make -f deployments/container/Makefile build push \
  IMAGE_NAME=$REGISTRY/$IMAGE_NAME \
  VERSION=$VERSION

# Install with Helm
helm upgrade -i runtime-spec-dra-driver deployments/helm/runtime-spec-dra-driver/ \
  --set image.pullPolicy=Always \
  --set image.repository=$REGISTRY/$IMAGE_NAME \
  --set image.tag=$VERSION \
  --set nri.enabled=true

# Test with example
kubectl apply -f demo/
```

## Development

### Building

```bash
# Build container image
make -f deployments/container/Makefile build

# Run tests
make test
```

### E2E Testing

```bash
# Run E2E tests with KinD
make test-e2e
```
