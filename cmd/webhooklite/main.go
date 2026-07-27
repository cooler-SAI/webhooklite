package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
	webhookRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhooklite_requests_total",
			Help: "Total number of admission review requests processed by webhooklite.",
		},
		[]string{"result"}, // Labels: "allowed" or "denied"
	)
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

		if !strings.Contains(image, "/") || !strings.Contains(registry, ".") {
			registry = "docker.io"
		}

		allowedRegistry := slices.Contains(allowedRegistries, registry)

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
		webhookRequestsTotal.WithLabelValues("denied").Inc()
	} else {
		log.Printf("✅ ALLOWED: %s", podName)
		webhookRequestsTotal.WithLabelValues("allowed").Inc()
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
	_, _ = fmt.Fprintf(w, "  /healthz - health check improved\n")
	_, _ = fmt.Fprintf(w, "  /metrics - prometheus metrics\n")
	_, _ = fmt.Fprintf(w, "  /validate - admission webhook\n")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", handleValidate)
	mux.HandleFunc("/healthz", handleHealth)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", handleRoot)

	srv := &http.Server{
		Addr:    ":8443",
		Handler: mux,
	}

	certFile := "/certs/tls.crt"
	keyFile := "/certs/tls.key"

	go func() {
		log.Printf("🔐 HTTPS server starting on port 8443")
		log.Printf("📜 Cert: %s, Key: %s", certFile, keyFile)
		log.Printf("📡 Endpoints: /healthz, /metrics, /validate")

		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("⚠️ Shutdown signal received, starting graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Graceful shutdown failed: %v", err)
	} else {
		log.Printf("✅ Server stopped cleanly")
	}
}
