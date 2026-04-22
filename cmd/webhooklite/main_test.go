package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestHandleValidate_RejectPrivileged(t *testing.T) {
	// Create pod with privileged: true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					SecurityContext: &corev1.SecurityContext{
						Privileged: boolPtr(true),
					},
				},
			},
		},
	}

	req := createAdmissionRequest(pod)
	rec := httptest.NewRecorder()
	handleValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response admissionv1.AdmissionReview
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Response.Allowed {
		t.Error("Expected pod to be rejected (privileged), but it was allowed")
	}
}

func TestHandleValidate_RejectLatestTag(t *testing.T) {
	// Create pod with latest tag
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "latest-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot:             boolPtr(true),
						AllowPrivilegeEscalation: boolPtr(false),
					},
				},
			},
		},
	}

	req := createAdmissionRequest(pod)
	rec := httptest.NewRecorder()
	handleValidate(rec, req)

	var response admissionv1.AdmissionReview
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Response.Allowed {
		t.Error("Expected pod to be rejected (latest tag), but it was allowed")
	}
}

func TestHandleValidate_RejectNoResourceLimits(t *testing.T) {
	// Create pod without resource limits
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "no limits-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.25",
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot:             boolPtr(true),
						AllowPrivilegeEscalation: boolPtr(false),
					},
				},
			},
		},
	}

	req := createAdmissionRequest(pod)
	rec := httptest.NewRecorder()
	handleValidate(rec, req)

	var response admissionv1.AdmissionReview
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Response.Allowed {
		t.Error("Expected pod to be rejected (no resource limits), but it was allowed")
	}
}

func TestHandleValidate_AcceptGoodPod(t *testing.T) {
	// Create right pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "good-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.25",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu":    resource.MustParse("100m"),
							"memory": resource.MustParse("128Mi"),
						},
						Requests: corev1.ResourceList{
							"cpu":    resource.MustParse("50m"),
							"memory": resource.MustParse("64Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot:             boolPtr(true),
						AllowPrivilegeEscalation: boolPtr(false),
					},
				},
			},
		},
	}

	req := createAdmissionRequest(pod)
	rec := httptest.NewRecorder()
	handleValidate(rec, req)

	var response admissionv1.AdmissionReview
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Response.Allowed {
		t.Errorf("Expected pod to be allowed, got rejected: %s", response.Response.Result.Message)
	}
}

// Advance functions
func boolPtr(b bool) *bool { return &b }

func createAdmissionRequest(pod *corev1.Pod) *http.Request {
	podBytes, _ := json.Marshal(pod)
	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
		},
	}
	body, _ := json.Marshal(review)
	return httptest.NewRequest("POST", "/validate", bytes.NewReader(body))
}
