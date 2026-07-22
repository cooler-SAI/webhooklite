$Namespace = "default"
$Service = "webhooklite"

Write-Host "1. Generating certs inside Docker (fixing Windows CRLF line endings)..." -ForegroundColor Cyan

$shScript = @"
apk add --no-cache openssl
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -subj "/CN=$Service.$Namespace.svc" -days 3650 -out ca.crt
openssl genrsa -out tls.key 2048
openssl req -new -key tls.key -subj "/CN=$Service.$Namespace.svc" -out server.csr
echo "subjectAltName=DNS:$Service.$Namespace.svc,DNS:$Service.$Namespace.svc.cluster.local" > ext.cnf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 3650 -extfile ext.cnf
"@

# Жестко конвертируем виндовые переносы строк в линуксовые
$shScript = $shScript -replace "`r`n", "`n"

# Передаем скрипт напрямую в контейнер через stdin (флаг -i)
$shScript | docker run -i --rm -v "$($PWD.Path):/certs" -w /certs alpine:latest sh

Write-Host "2. Updating K8s Secret..." -ForegroundColor Cyan
kubectl delete secret webhooklite-certs -n $Namespace --ignore-not-found
kubectl create secret tls webhooklite-certs --cert=tls.crt --key=tls.key -n $Namespace

Write-Host "3. Restarting webhook pod to load new certs..." -ForegroundColor Cyan
kubectl rollout restart deploy/webhooklite
Start-Sleep -Seconds 5 # Даем поду время подняться

Write-Host "4. Applying Webhook Configuration (with Fail policy & safe namespaceSelector)..." -ForegroundColor Cyan
$caPath = Join-Path (Get-Location).Path "ca.crt"
$caBundle = [Convert]::ToBase64String([IO.File]::ReadAllBytes($caPath))

$webhookYaml = @"
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: webhooklite-validator
webhooks:
  - name: validate.webhooklite.k8s.local
    rules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["pods"]
        scope: "Namespaced"
    namespaceSelector:
      matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: NotIn
          values: ["kube-system"]
    clientConfig:
      service:
        namespace: default
        name: webhooklite
        path: /validate
      caBundle: $caBundle
    admissionReviewVersions: ["v1"]
    sideEffects: None
    timeoutSeconds: 5
    failurePolicy: Fail
"@

$webhookYaml | kubectl apply -f -
Write-Host "Done! The matrix is reloaded." -ForegroundColor Green