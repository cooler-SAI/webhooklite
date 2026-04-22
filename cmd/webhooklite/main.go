package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func handleValidate(w http.ResponseWriter, r *http.Request) {
	log.Println("🔍 Webhook called for validation")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Error reading body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("❌ Error closing body: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}(r.Body)

	var admissionReview admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		log.Printf("❌ Error decoding JSON: %v", err)
		http.Error(w, fmt.Sprintf("JSON decode error: %v", err), http.StatusBadRequest)
		return
	}

	if admissionReview.Request == nil {
		log.Printf("❌ AdmissionReview.Request is nil")
		http.Error(w, "AdmissionReview.Request is nil", http.StatusBadRequest)
		return
	}

	var pod corev1.Pod
	if err := json.Unmarshal(admissionReview.Request.Object.Raw, &pod); err != nil {
		log.Printf("❌ Error unmarshaling pod: %v", err)
	}

	allowed := true
	var violations []string

	podName := pod.Name
	if podName == "" && admissionReview.Request.Name != "" {
		podName = admissionReview.Request.Name
	}

	// ========== RULE 1: Deny privileged containers ==========
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext != nil && container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			allowed = false
			violations = append(violations, fmt.Sprintf("Container '%s' is privileged (not allowed)", container.Name))
			log.Printf("❌ REJECTED: %s - container '%s' is privileged", podName, container.Name)
		}
	}

	// ========== RULE 2: Deny latest tags ==========
	for _, container := range pod.Spec.Containers {
		image := container.Image
		if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
			allowed = false
			violations = append(violations, fmt.Sprintf("Container '%s' uses 'latest' tag or no tag (must specify version)", container.Name))
			log.Printf("❌ REJECTED: %s - container '%s' uses 'latest' or untagged", podName, container.Name)
		}
	}

	// ========== RULE 3: Require resource limits ==========
	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits == nil || len(container.Resources.Limits) == 0 {
			allowed = false
			violations = append(violations, fmt.Sprintf("Container '%s' must have resource limits defined", container.Name))
			log.Printf("❌ REJECTED: %s - container '%s' missing resource limits", podName, container.Name)
		}
	}

	// ========== RULE 4: runAsNonRoot is required ==========
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext == nil || container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
			allowed = false
			violations = append(violations, fmt.Sprintf("Container '%s' must set runAsNonRoot=true", container.Name))
			log.Printf("❌ REJECTED: %s - container '%s' missing runAsNonRoot", podName, container.Name)
		}
	}

	// ========== RULE 5: Deny allowPrivilegeEscalation ==========
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext != nil && container.SecurityContext.AllowPrivilegeEscalation != nil && *container.SecurityContext.AllowPrivilegeEscalation {
			allowed = false
			violations = append(violations, fmt.Sprintf("Container '%s' has allowPrivilegeEscalation=true (not allowed)", container.Name))
			log.Printf("❌ REJECTED: %s - container '%s' allows privilege escalation", podName, container.Name)
		}
	}

	// ========== RULE 6: Deny hostNetwork and hostPID ==========
	if pod.Spec.HostNetwork {
		allowed = false
		violations = append(violations, "HostNetwork is not allowed")
		log.Printf("❌ REJECTED: %s - HostNetwork=true", podName)
	}
	if pod.Spec.HostPID {
		allowed = false
		violations = append(violations, "HostPID is not allowed")
		log.Printf("❌ REJECTED: %s - HostPID=true", podName)
	}

	// ========== RULE 7: Check allowed registries ==========
	allowedRegistries := []string{"docker.io", "registry.k8s.io", "gcr.io", "ghcr.io"}
	for _, container := range pod.Spec.Containers {
		image := container.Image
		registry := strings.Split(image, "/")[0]

		// If no registry (nginx:1.25) → docker.io
		if !strings.Contains(image, "/") || !strings.Contains(registry, ".") {
			registry = "docker.io"
		}

		allowedRegistry := false
		for _, allowed := range allowedRegistries {
			if registry == allowed {
				allowedRegistry = true
				break
			}
		}

		if !allowedRegistry {
			allowed = false
			violations = append(violations, fmt.Sprintf("Image '%s' from registry '%s' is not in allowed list: %v", image, registry, allowedRegistries))
			log.Printf("❌ REJECTED: %s - registry '%s' not in allowed list", podName, registry)
		}
	}

	// ========== RULE 8: Deny docker.socket mounting ==========
	for _, volume := range pod.Spec.Volumes {
		if volume.HostPath != nil && volume.HostPath.Path == "/var/run/docker.sock" {
			allowed = false
			violations = append(violations, "Mounting docker.socket is not allowed")
			log.Printf("❌ REJECTED: %s - docker.socket mounted", podName)
		}
	}

	message := ""
	if !allowed {
		message = strings.Join(violations, "; ")
		log.Printf("❌ REJECTED: %s - %s", podName, message)
	} else {
		log.Printf("✅ ALLOWED: %s", podName)
	}

	admissionResponse := &admissionv1.AdmissionResponse{
		UID:     admissionReview.Request.UID,
		Allowed: allowed,
	}

	if !allowed {
		admissionResponse.Result = &metav1.Status{
			Message: message,
			Code:    http.StatusForbidden,
		}
	}

	responseReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: admissionResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(responseReview); err != nil {
		log.Printf("❌ Error encoding response: %v", err)
		http.Error(w, fmt.Sprintf("Response encoding error: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "ok")
}

func handleRoot(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintf(w, "webhooklite is running\n")
	_, _ = fmt.Fprintf(w, "Endpoints:\n")
	_, _ = fmt.Fprintf(w, "  /health - health check\n")
	_, _ = fmt.Fprintf(w, "  /validate - admission webhook\n")
}

func main() {
	// Register handlers
	http.HandleFunc("/validate", handleValidate)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/", handleRoot)

	certFile := "/certs/tls.crt"
	keyFile := "/certs/tls.key"

	log.Printf("🔐 HTTPS server starting on port 8443")
	log.Printf("📜 Cert: %s, Key: %s", certFile, keyFile)
	log.Printf("📡 Endpoints: /health, /validate")

	if err := http.ListenAndServeTLS(":8443", certFile, keyFile, nil); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
