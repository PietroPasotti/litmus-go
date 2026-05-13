# Juju Chaos Plugin for LitmusChaos — Project Plan

## Goal

Build a Juju plugin for LitmusChaos that enables chaos experiments targeting Juju-managed applications. Examples:
- "If I remove this relation and add it back, the charm reaches active status within 5 minutes"
- "If I add this relation while I kill this pod, I can reach this endpoint over TLS"

## Architecture

- **No plugin system in Litmus** — extensibility comes through ChaosHub (git repos of fault YAML definitions) and the go-runner image (compiled experiments)
- Experiments are added directly to a **fork of `litmus-go`**, compiled into the `go-runner` binary
- A separate **`juju-chaos-charts`** repo provides the ChaosHub YAML definitions
- Juju operations use the **`juju` CLI via `os/exec`** (not the Go API, due to k8s client-go dependency conflict: litmus needs v0.21.2, juju needs v0.34.x)
- Juju credentials are passed via K8s Secret mounted at `/tmp/juju-credentials.yaml`

## Fault Injection Flow

```
GraphQL Server → WebSocket → Subscriber Agent → kubectl apply → Argo Workflow
→ litmus-checker applies ChaosEngine CR → chaos-operator spawns chaos-runner
→ go-runner pod executes the experiment
```

## 8 Implemented Fault Types

| Fault | Description |
|-------|-------------|
| `juju-integrate` | Add a relation between two endpoints |
| `juju-remove-relation` | Remove a relation between two endpoints |
| `juju-add-unit` | Scale up an application |
| `juju-remove-unit` | Scale down an application |
| `juju-add-app` | Deploy a new application |
| `juju-remove-app` | Remove an application |
| `juju-set-config` | Change application config |
| `juju-run-action` | Execute a Juju action on a unit |

## Experiment Pattern (per litmus-go conventions)

Each experiment has 4 files:
1. `pkg/juju/<fault>/types/types.go` — ExperimentDetails struct
2. `pkg/juju/<fault>/environment/environment.go` — GetENV() reads env vars
3. `chaoslib/litmus/juju-<fault>/lib/*.go` — `Prepare*()` function with signal handling, abort watchers
4. `experiments/juju/juju-<fault>/experiment/*.go` — Orchestrator (lifecycle: GetENV → InitChaosVars → SOT → PreChaos probes → inject → PostChaos probes → EOT → verdict events)

Shared code:
- `pkg/cloud/juju/common/common.go` — JujuCredentials, JujuClient (CLI wrapper)
- `pkg/cloud/juju/operations/operations.go` — All 8 operations + WaitForApplicationStatus
- `pkg/juju/types/types.go` — JujuCommonDetails shared struct

All 8 experiments are registered in `bin/experiment/experiment.go` switch statement.

## Repos

| Repo | Path (on host) | Mount in VM | Description |
|------|----------------|-------------|-------------|
| litmus-go (fork) | `~/canonical/litmus-go` | `/home/ubuntu/litmus-go` | Go experiments + Dockerfile |
| juju-chaos-charts | `~/canonical/juju-chaos-charts` | `/home/ubuntu/juju-chaos-charts` | ChaosHub YAML definitions |
| litmus (reference) | `~/canonical/litmus` | `/home/ubuntu/litmus` | LitmusChaos control plane (read-only reference) |

## Completed

- [x] Explored litmus codebase — understood architecture, probe system, experiment flow
- [x] Created `juju-chaos-charts` — 26 YAML files (category CSV, package.yaml, 8 fault dirs with CSV/fault.yaml/engine.yaml)
- [x] Added all 8 Juju experiments to litmus-go fork (types, env, chaoslib, experiment, registration)
- [x] Shared juju client code (`pkg/cloud/juju/common/` and `pkg/cloud/juju/operations/`)
- [x] Binary compiles: `CGO_ENABLED=0 go build ./bin/experiment/` → 61MB binary
- [x] Dockerfile modified to include juju 3.6.1 CLI (from Launchpad tarball)
- [x] Docker image `juju-go-runner:latest` built and verified
- [x] Fixed env var mismatch: renamed `MODEL` → `JUJU_MODEL_UUID` in all chaos-charts YAMLs

## TODO — Next Steps

### 1. Push the Docker image to a registry
The `juju-go-runner:latest` image needs to be pushed to a registry accessible by the K8s cluster where Litmus runs.

```bash
# Tag and push (adjust registry as needed)
docker tag juju-go-runner:latest ghcr.io/canonical/juju-go-runner:latest
docker push ghcr.io/canonical/juju-go-runner:latest
```

### 2. Create K8s Secret with Juju credentials
The experiments read credentials from `/tmp/juju-credentials.yaml` (mounted via K8s Secret).

```yaml
# juju-credentials.yaml
controller-addr: "10.0.0.1:17070"
ca-cert: |
  -----BEGIN CERTIFICATE-----
  ...
  -----END CERTIFICATE-----
username: "admin"
password: "your-password"
model-uuid: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

```bash
kubectl create secret generic juju-credentials \
  --from-file=juju-credentials.yaml=/path/to/juju-credentials.yaml \
  -n litmus
```

Then update the ChaosEngine/fault YAMLs to mount this secret at `/tmp/juju-credentials.yaml` in the experiment pod.

### 3. Register ChaosHub in Litmus
Point Litmus at the `juju-chaos-charts` git repo as a custom ChaosHub (via the Litmus web UI or GraphQL API).

### 4. Update go-runner image reference
The fault.yaml files in `juju-chaos-charts` reference the go-runner image. Update them to point to your custom `juju-go-runner` image instead of the default `litmuschaos/go-runner`.

### 5. Run a fault
Create a ChaosEngine CR targeting one of the 8 Juju experiments and apply it to the cluster.

### 6. Upstream prep
- Write tests for the juju experiments
- Document the credential setup
- PR to litmuschaos/litmus-go and litmuschaos/chaos-charts

## Key Files Quick Reference

```
litmus-go/
├── bin/experiment/experiment.go          # Switch statement registering all experiments
├── build/Dockerfile                       # Modified: includes juju CLI
├── pkg/cloud/juju/
│   ├── common/common.go                   # JujuCredentials, JujuClient
│   └── operations/operations.go           # All 8 juju operations
├── pkg/juju/
│   ├── types/types.go                     # JujuCommonDetails
│   ├── integrate/                         # types/ + environment/
│   ├── remove-relation/
│   ├── add-unit/
│   ├── remove-unit/
│   ├── add-app/
│   ├── remove-app/
│   ├── set-config/
│   └── run-action/
├── chaoslib/litmus/juju-*/lib/            # 8 Prepare* functions
└── experiments/juju/juju-*/experiment/    # 8 orchestrators

juju-chaos-charts/
└── faults/juju/
    ├── juju.chartserviceversion.yaml
    ├── juju.package.yaml
    └── juju-{integrate,...}/
        ├── *.chartserviceversion.yaml
        ├── fault.yaml
        └── engine.yaml
```
