# webhooklite

[![Go Version](https://img.shields.io/badge/Go-1.26.3-blue.svg)](https://golang.org/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.34-blue.svg)](https://kubernetes.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Production-ready Kubernetes admission webhook that validates pods BEFORE they enter the cluster. Enforces 8 security policies to prevent insecure workloads from running.

## Overview

**webhooklite** is a lightweight but powerful Kubernetes admission webhook that intercepts pod creation requests and validates them against security best practices. Unlike security scanners that detect issues after deployment, webhooklite actively blocks non-compliant pods at admission time.

### Why use webhooklite?

- **Prevents container escapes** - Blocks privileged containers and host access
- **Enforces best practices** - Requires resource limits and non-root users
- **Supply chain security** - Restricts image registries and blocks latest tags
- **Zero runtime overhead** - No sidecars or agents, pure admission control

## 8 security rules

| # | Rule | Blocks | Why |
|---|------|--------|-----|
| 1 | No privileged containers | `privileged: true` | Prevents container escape |
| 2 | No latest tags | `image: nginx:latest` | Ensures version pinning |
| 3 | Resource limits required | Missing `resources.limits` | Prevents DoS attacks |
| 4 | runAsNonRoot required | `runAsNonRoot: false` | Reduces attack surface |
| 5 | No privilege escalation | `allowPrivilegeEscalation: true` | Blocks CAP_SYS_ADMIN |
| 6 | No host access | `hostNetwork` / `hostPID` | Isolates from host |
| 7 | Allowed registries only | Unknown registries | Prevents supply chain attacks |
| 8 | No docker.socket | Mounting `/var/run/docker.sock` | Blocks container breakout |

## Quick start

```bash
git clone https://github.com/cooler-SAI/webhooklite.git
cd webhooklite

# Generate TLS certificates
./scripts/generate-certs.sh  # Linux/Mac
.\scripts\generate-certs.ps1 # Windows

# Deploy webhook
kubectl apply -f deploy/
```

## Build from source

```bash
go mod init webhooklite
go get k8s.io/api@v0.28.0
go get k8s.io/apimachinery@v0.28.0
go build -o webhook webhook.go
```

## Test it works

### ❌ Should be BLOCKED

```bash
# Rule 1: Privileged container
kubectl run bad-priv --image=nginx:1.21 --privileged=true

# Rule 2: Latest tag
kubectl run bad-latest --image=nginx:latest

# Rule 3: No resource limits
kubectl run bad-nolimits --image=nginx:1.21

# Rule 4: Root user
kubectl run bad-root --image=nginx:1.21 --overrides='{"spec":{"securityContext":{"runAsNonRoot":false}}}'

# Rule 6: Host network
kubectl run bad-hostnet --image=nginx:1.21 --overrides='{"spec":{"hostNetwork":true}}'
```

### ✅ Should be ALLOWED

```bash
kubectl run good-pod --image=nginx:1.21 --overrides='{
  "spec": {
    "securityContext": {"runAsNonRoot": true},
    "containers": [{
      "name": "nginx",
      "image": "nginx:1.21",
      "resources": {"limits": {"cpu": "100m", "memory": "128Mi"}}
    }]
  }
}'
```

