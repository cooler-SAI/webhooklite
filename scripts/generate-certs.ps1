$Namespace = "default"
$Service = "webhooklite"

Write-Host "Generating certificates for $Service in namespace $Namespace..." -ForegroundColor Cyan

# Создаем временную директорию для генерации, чтобы не засорять корень проекта
$certDir = "$PSScriptRoot\..\certs"
if (!(Test-Path $certDir)) {
    New-Item -ItemType Directory -Path $certDir | Out-Null
}

$caKey = "$certDir\ca.key"
$caCert = "$certDir\ca.crt"
$tlsKey = "$certDir\tls.key"
$tlsCert = "$certDir\tls.crt"
$serverCsr = "$certDir\server.csr"

# 1. Create CA private key
openssl genrsa -out $caKey 2048

# 2. Create CA certificate
openssl req -x509 -new -nodes -key $caKey -subj "/CN=$Service.$Namespace.svc" -days 3650 -out $caCert

# 3. Create server private key
openssl genrsa -out $tlsKey 2048

# 4. Create Certificate Signing Request (CSR) for the server
$san = "subjectAltName=DNS:$Service.$Namespace.svc,DNS:$Service.$Namespace.svc.cluster.local"
openssl req -new -key $tlsKey -subj "/CN=$Service.$Namespace.svc" -addext $san -out $serverCsr

# 5. Sign the server certificate with our CA
$extFile = [System.IO.Path]::GetTempFileName()
@"
$san
"@ | Out-File -FilePath $extFile -Encoding ascii

openssl x509 -req -in $serverCsr -CA $caCert -CAkey $caKey -CAcreateserial -out $tlsCert -days 3650 -extfile $extFile

# Clean up temp file and CSR
Remove-Item $extFile
Remove-Item $serverCsr
Remove-Item "$certDir\ca.srl" -ErrorAction SilentlyContinue

Write-Host "Creating Kubernetes Secret..." -ForegroundColor Cyan

# 6. Create Secret in Kubernetes using generated certs from certs/ folder
kubectl create secret tls webhooklite-certs --cert=$tlsCert --key=$tlsKey -n $Namespace --dry-run=client -o yaml | kubectl apply -f -

Write-Host "Done! Certificates generated in certs/ and Secret created." -ForegroundColor Green