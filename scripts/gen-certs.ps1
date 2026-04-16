# scripts\gen-certs.ps1
param(
    [string]$Namespace = "webhook-system",
    [string]$ServiceName = "webhook-service"
)

Write-Host "🎯 Generating certificates for $ServiceName.$Namespace.svc" -ForegroundColor Cyan

# Create certs directory
$certsDir = Join-Path $PSScriptRoot "..\certs"
New-Item -ItemType Directory -Path $certsDir -Force | Out-Null

# Create openssl config
$serviceName = "$ServiceName.$Namespace.svc"
$opensslCnf = @"
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = $serviceName

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = $ServiceName
DNS.2 = $ServiceName.$Namespace
DNS.3 = $ServiceName.$Namespace.svc
DNS.4 = $ServiceName.$Namespace.svc.cluster.local
DNS.5 = localhost
IP.1 = 127.0.0.1
"@

$opensslCnf | Out-File -FilePath "$certsDir\openssl.cnf" -Encoding ascii

Write-Host "🔑 Generating private key..." -ForegroundColor Yellow
docker run --rm -v "${certsDir}:/certs" alpine sh -c "apk add --no-cache openssl && openssl genrsa -out /certs/tls.key 2048"

Write-Host "📝 Generating certificate..." -ForegroundColor Yellow
docker run --rm -v "${certsDir}:/certs" alpine sh -c "apk add --no-cache openssl && openssl req -new -x509 -days 365 -key /certs/tls.key -out /certs/tls.crt -config /certs/openssl.cnf"

Write-Host "✅ Certificates created in $certsDir" -ForegroundColor Green

# Create Kubernetes secret
Write-Host "📦 Creating Kubernetes secret..." -ForegroundColor Yellow
kubectl create secret tls webhook-certs `
    --cert="$certsDir\tls.crt" `
    --key="$certsDir\tls.key" `
    -n $Namespace `
    --dry-run=client -o yaml | kubectl apply -f -

# Get caBundle for validator
$caBundle = [Convert]::ToBase64String([System.IO.File]::ReadAllBytes("$certsDir\tls.crt"))
Write-Host "`n🔑 caBundle for validator:" -ForegroundColor Cyan
Write-Host $caBundle

Write-Host "`n✅ Done!" -ForegroundColor Green