# webhooklite

Kubernetes admission webhook with 8 security policies. Blocks insecure pods BEFORE they enter the cluster.

## 8 rules

| # | Blocks |
|---|--------|
| 1 | `privileged: true` |
| 2 | `image: nginx:latest` |
| 3 | Missing `resources.limits` |
| 4 | `runAsNonRoot: false` |
| 5 | `allowPrivilegeEscalation: true` |
| 6 | `hostNetwork: true` / `hostPID: true` |
| 7 | Unknown registries |
| 8 | Mounting `/var/run/docker.sock` |

## Quick start

```bash
git clone https://github.com/cooler-SAI/webhooklite.git
cd webhooklite

./scripts/generate-certs.sh  # Linux/Mac
.\scripts\generate-certs.ps1 # Windows

kubectl apply -f deploy/
kubectl get pods -n webhook-system
