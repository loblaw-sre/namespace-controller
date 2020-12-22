# Development and CD flow

We use kustomize + kind to ensure rapid iteration.

## Before you start
  - Ensure you have the following installed:
    - kustomize (ensured by make)
    - controller-gen (ensured by make)
    - kind
    - kapp
    - kubectl

## Architecture
Our kustomize manifests are configured with each individual component in
individual folders. The root of the kustomize manifests folder is in the
`deploy` folder, and the entrypoints to kustomize are inside the `overlays`
folder inside of the root.

### Local environment
facilitated by `go run`, note that this environment does not and cannot be
configured to install webhooks, since there is a dependency on cert-manager.
This environment is not recommended as the mutating webhook is an essential
part of the business logic.

### Kind environment
Our kind environment consists of certain workarounds in order to facilitate offline development.

#### Usage
Key make commands
- `make up` - bring kind cluster up
- `make down` - destroy kind cluster
- `make redeploy` - For updating changes on kind


#### Billing controller disabled by default
The deployment of billing controller is disabled by default in order to
facilitate easier development. If development on billing controller is
desired, it must be manually configured to be switched on. [Instructions on
how to do that](guides/install-billingcontroller-on-local.md)

### Dev environment
Dev environments assume workload identity on GKE has access to the required
bigquery tables and datasets.

If you're looking to deploy onto a playground GKE, edit the dataset/table
names to point back to your playground project bigquery
