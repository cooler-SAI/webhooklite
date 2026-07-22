$Namespace = "default"
$Service = "webhooklite"

Write-Host "Generating certificates for $Service in namespace $Namespace..." -ForegroundColor Cyan

# 1. Create CA private key
openssl genrsa -out ca.key 2048

# 2. Create CA certificate
openssl req -x509 -new -nodes -key ca.key -subj "/CN=$Service.$Namespace.svc" -days 3650 -out ca.crt

# 3. Create server private key
openssl genrsa -out tls.key 2048

# 4. Create Certificate Signing Request (CSR) for the server
# IMPORTANT: Subject Alternative Name (SAN) is required for Go 1.15+
$san = "subjectAltName=DNS:$Service.$Namespace.svc,DNS:$Service.$Namespace.svc.cluster.local"
openssl req -new -key tls.key -subj "/CN=$Service.$Namespace.svc" -addext $san -out server.csr

# 5. Sign the server certificate with our CA
# Create a temporary config file for the SAN extension
$extFile = [System.IO.Path]::GetTempFileName()
@"
$san
"@ | Out-File -FilePath $extFile -Encoding ascii

openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 3650 -extfile $extFile

# Clean up temp file
Remove-Item $extFile

Write-Host "Creating Kubernetes Secret..." -ForegroundColor Cyan

# 6. Create Secret in Kubernetes
kubectl create secret tls webhooklite-certs --cert=tls.crt --key=tls.key -n $Namespace --dry-run=client -o yaml | kubectl apply -f -

Write-Host "Done! Certificates generated and Secret created." -ForegroundColor Green