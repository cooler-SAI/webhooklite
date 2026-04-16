# Go Kubernetes Security Lab

[![Go Version](https://img.shields.io/badge/Go-1.26.3-blue.svg)](https://golang.org/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.34-blue.svg)](https://kubernetes.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A hands-on security lab demonstrating how to build and deploy secure Go applications on Kubernetes with modern security best practices.

## 🎯 Overview

This repository is a comprehensive security lab that provides practical experience in securing containerized workloads in a Kubernetes environment. It includes multiple services demonstrating security concepts from application-level to infrastructure-level enforcement.

## 📦 Project Components

### Core Services

| Service | Description | Language |
|---------|-------------|----------|
| **`websecure`** | Go web server with JWT auth, rate limiting, security headers, and XSS protection | Go |
| **`emuserver`** | Chaos engineering tool for testing resilience (random delays/errors) | Go |
| **`webhooklite`** | Production-ready admission webhook with 8 security policies | Go |
| **`sentinel`** | Admission webhook blocking privileged containers | Go |
| **`sac`** | Russian-language admission webhook example | Go |

### `webhooklite` — Admission Webhook (Main Focus)

A lightweight but powerful Kubernetes admission webhook that validates pods **BEFORE** they enter the cluster.

#### 🔒 Security Rules Implemented

| Rule | What It Blocks |
|------|----------------|
| ❌ No privileged containers | `privileged: true` |
| ❌ No latest tags | `image: nginx:latest` |
| ❌ Resource limits required | Missing `resources.limits` |
| ❌ runAsNonRoot required | `runAsNonRoot: false` |
| ❌ No privilege escalation | `allowPrivilegeEscalation: true` |
| ❌ No host access | `hostNetwork: true` or `hostPID: true` |
| ❌ Allowed registries only | Unknown image registries |
| ❌ No docker.socket | Mounting `/var/run/docker.sock` |

## 🛡️ Security Features Demonstrated

### Application-Level Security
- **JWT Authentication** — Secure endpoint protection
- **Rate Limiting** — DoS attack prevention
- **Security Headers** — XSS, clickjacking protection
- **RBAC** — Role-based access control

### Infrastructure-Level Security
- **Admission Webhooks** — Custom cluster policies
- **Hardened Dockerfiles** — Multi-stage, non-root builds
- **Secure K8s Deployments** — Strict securityContext
- **TLS Certificates** — Self-signed with proper SANs
- **Network Policies** — Service isolation

## 🚀 Quick Start

### Prerequisites
- Go 1.26+
- Docker Desktop with Kubernetes enabled
- kubectl
- PowerShell 7+

### Deploy Webhooklite

```powershell
# Clone repository
git clone https://github.com/cooler-SAI/webhooklite.git
cd webhooklite

# Generate certificates and deploy everything
.\scripts\deploy.ps1