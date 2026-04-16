# scripts\deploy.ps1
param(
    [string]$Namespace = "webhook-system"
)

Write-Host "🚀 Deploying webhook to namespace $Namespace" -ForegroundColor Cyan

# 1. Create namespace
Write-Host "📦 Creating namespace..." -ForegroundColor Yellow
kubectl apply -f deployments\00-namespace.yaml

# 2. Generate certificates and secret
Write-Host "🔐 Generating certificates..." -ForegroundColor Yellow
& "$PSScriptRoot\gen-certs.ps1"

# 3. Apply RBAC
Write-Host "🔑 Applying RBAC..." -ForegroundColor Yellow
kubectl apply -f deployments\01-rbac.yaml

# 4. Build and load image
Write-Host "🛠️ Building Docker image..." -ForegroundColor Yellow
docker build -t webhooklite:latest -f build\Dockerfile .

# For Kind cluster:
# kind load docker-image webhooklite:latest --name your-cluster-name

# 5. Apply deployment
Write-Host "🚀 Applying deployment..." -ForegroundColor Yellow
kubectl apply -f deployments\03-deployment.yaml

# 6. Apply service
Write-Host "🌐 Applying service..." -ForegroundColor Yellow
kubectl apply -f deployments\04-service.yaml

# 7. Wait for pod
Write-Host "⏳ Waiting for pod to be ready..." -ForegroundColor Yellow
kubectl wait --for=condition=ready pod -l app=webhook -n $Namespace --timeout=60s

# 8. Get caBundle and update validator
Write-Host "🔑 Updating validator with caBundle..." -ForegroundColor Yellow
$caBundle = [Convert]::ToBase64String([System.IO.File]::ReadAllBytes("certs\tls.crt"))
$validatorPath = "deployments\05-validator.yaml"
$content = Get-Content $validatorPath -Raw
$content = $content -replace 'caBundle: ""', "caBundle: $caBundle"
$content | Set-Content $validatorPath

# 9. Apply validator
kubectl apply -f $validatorPath

Write-Host "✅ Deployment complete!" -ForegroundColor Green
Write-Host ""
Write-Host "📊 Check status: kubectl get all -n $Namespace"
Write-Host "📝 View logs: kubectl logs -l app=webhook -n $Namespace"