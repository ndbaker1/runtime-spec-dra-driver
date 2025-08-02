# OCI RuntimeSpec DRA Driver

An 🧪 experimental DRA driver that allows setting arbitrary oci runtime spec fields
into pods via configuration associated with a `ResourceClaim`.

It abuses the CDI container edits API to inject an oci hook into the container
creation lifecycle which acts based on a user-provided config.

## Quickstart

```bash
export REGISTRY=
export VERSION=
export DRIVER_NAME=

make generate
make -f deployments/container/Makefile build push

helm upgrade -i runtime-spec-dra-driver deployments/helm/runtime-spec-dra-driver/ \
  --set image.pullPolicy=Always \
  --set image.repository=$REGISTRY/$DRIVER_NAME \
  --set image.tag=$VERSION \

# attempt some of the test cases
kubectl apply -f demo/
```
