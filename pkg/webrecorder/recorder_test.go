package webrecorder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewWebhookEventRecorder(t *testing.T) {
	webhookURL := "https://example.com/webhook"
	eventID := "test-event"
	clusterID := "test-cluster"

	recorder := NewWebhookEventRecorder(webhookURL, eventID, clusterID)
	if recorder == nil {
		t.Error("NewWebhookEventRecorder() returned nil")
	}

	webRecorder, ok := recorder.(*WebhookEventRecorder)
	if !ok {
		t.Error("NewWebhookEventRecorder() did not return WebhookEventRecorder type")
	}

	if webRecorder.webhookURL != webhookURL {
		t.Errorf("webhookURL = %s, want %s", webRecorder.webhookURL, webhookURL)
	}
	if webRecorder.eventID != eventID {
		t.Errorf("eventID = %s, want %s", webRecorder.eventID, eventID)
	}
	if webRecorder.clusterID != clusterID {
		t.Errorf("clusterID = %s, want %s", webRecorder.clusterID, clusterID)
	}
	if webRecorder.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestWebhookEventRecorderAnnotatedEventf(t *testing.T) {
	// Create a test HTTP server that captures the webhook payload
	var lastPayload *WebhookEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			lastPayload = &payload
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookRecorder := NewWebhookEventRecorder(
		server.URL,
		"test-event",
		"test-cluster",
	).(*WebhookEventRecorder)

	obj := &testObject{name: "test-obj"}
	annotations := map[string]string{
		PhaseKey:  "TestPhase",
		StatusKey: StatusSuccess,
		StepKey:   "TestStep",
	}

	webhookRecorder.AnnotatedEventf(obj, annotations, "Normal", "TestReason", "Test message %s", "formatted")

	// Wait for webhook to be sent
	time.Sleep(200 * time.Millisecond)

	if lastPayload == nil {
		t.Fatal("Webhook payload was not received")
	}

	if lastPayload.Phase != "TestPhase" {
		t.Errorf("Phase = %s, want TestPhase", lastPayload.Phase)
	}
	if lastPayload.Status != StatusSuccess {
		t.Errorf("Status = %s, want %s", lastPayload.Status, StatusSuccess)
	}
	if lastPayload.Step != "TestStep" {
		t.Errorf("Step = %s, want TestStep", lastPayload.Step)
	}
	if lastPayload.ClusterID != "test-cluster" {
		t.Errorf("ClusterID = %s, want test-cluster", lastPayload.ClusterID)
	}
	if lastPayload.Message != "Test message formatted" {
		t.Errorf("Message = %s, want 'Test message formatted'", lastPayload.Message)
	}
}

func TestWebhookEventRecorderWithResourceChange(t *testing.T) {
	// Create a test HTTP server that captures the webhook payload
	var lastPayload *WebhookEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			lastPayload = &payload
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookRecorder := NewWebhookEventRecorder(
		server.URL,
		"test-event",
		"test-cluster",
	).(*WebhookEventRecorder)

	obj := &testObject{name: "test-obj"}
	annotations := map[string]string{
		PhaseKey:  "Scale",
		StatusKey: StatusInProgress,
		StepKey:   "UpdateReplicas",
	}

	oldReplicas := int32(3)
	newReplicas := int32(5)
	resourceChange := &ResourceChange{
		CPUBefore:      "1000m",
		CPUAfter:       "2000m",
		MemoryBefore:   "1Gi",
		MemoryAfter:    "2Gi",
		DiskBefore:     "10Gi",
		DiskAfter:      "20Gi",
		ReplicasBefore: &oldReplicas,
		ReplicasAfter:  &newReplicas,
	}

	webhookRecorder.AnnotatedEventfWithResourceChange(
		obj,
		annotations,
		resourceChange,
		"Normal",
		"ResourceChange",
		"Scaling from %d to %d replicas",
		oldReplicas,
		newReplicas,
	)

	// Wait for webhook to be sent
	time.Sleep(200 * time.Millisecond)

	if lastPayload == nil {
		t.Fatal("Webhook payload was not received")
	}

	if lastPayload.ResourceChange == nil {
		t.Fatal("ResourceChange was not included in payload")
	}

	if lastPayload.ResourceChange.CPUBefore != "1000m" {
		t.Errorf("CPUBefore = %s, want 1000m", lastPayload.ResourceChange.CPUBefore)
	}
	if lastPayload.ResourceChange.CPUAfter != "2000m" {
		t.Errorf("CPUAfter = %s, want 2000m", lastPayload.ResourceChange.CPUAfter)
	}
	if lastPayload.ResourceChange.MemoryBefore != "1Gi" {
		t.Errorf("MemoryBefore = %s, want 1Gi", lastPayload.ResourceChange.MemoryBefore)
	}
	if lastPayload.ResourceChange.MemoryAfter != "2Gi" {
		t.Errorf("MemoryAfter = %s, want 2Gi", lastPayload.ResourceChange.MemoryAfter)
	}
	if lastPayload.ResourceChange.DiskBefore != "10Gi" {
		t.Errorf("DiskBefore = %s, want 10Gi", lastPayload.ResourceChange.DiskBefore)
	}
	if lastPayload.ResourceChange.DiskAfter != "20Gi" {
		t.Errorf("DiskAfter = %s, want 20Gi", lastPayload.ResourceChange.DiskAfter)
	}
	if *lastPayload.ResourceChange.ReplicasBefore != oldReplicas {
		t.Errorf("ReplicasBefore = %d, want %d", *lastPayload.ResourceChange.ReplicasBefore, oldReplicas)
	}
	if *lastPayload.ResourceChange.ReplicasAfter != newReplicas {
		t.Errorf("ReplicasAfter = %d, want %d", *lastPayload.ResourceChange.ReplicasAfter, newReplicas)
	}
}

func TestWebhookWithEmptyURL(t *testing.T) {
	// Recorder with empty URL should not send webhooks
	webhookRecorder := NewWebhookEventRecorder(
		"",
		"test-event",
		"test-cluster",
	).(*WebhookEventRecorder)

	obj := &testObject{name: "test-obj"}
	annotations := map[string]string{
		PhaseKey:  "TestPhase",
		StatusKey: StatusSuccess,
		StepKey:   "TestStep",
	}

	// This should not panic or fail
	webhookRecorder.AnnotatedEventf(obj, annotations, "Normal", "TestReason", "Test message")

	// Give it a moment to complete
	time.Sleep(100 * time.Millisecond)
}

func TestWebhookRetry(t *testing.T) {
	t.Skip("Skipping retry test as it requires a long wait time")

	// Create a server that fails initially then succeeds
	var attemptCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookRecorder := NewWebhookEventRecorder(
		server.URL,
		"test-event",
		"test-cluster",
	).(*WebhookEventRecorder)

	obj := &testObject{name: "test-obj"}
	annotations := map[string]string{
		PhaseKey:  "TestPhase",
		StatusKey: StatusSuccess,
	}

	webhookRecorder.AnnotatedEventf(obj, annotations, "Normal", "TestReason", "Test message")

	// Wait for retries
	time.Sleep(2 * time.Second)

	finalCount := atomic.LoadInt32(&attemptCount)
	if finalCount < 3 {
		t.Errorf("Expected at least 3 attempts, got %d", finalCount)
	}
}

func TestWebhookEventConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"PhaseKey", PhaseKey, "infini.cloud/phase"},
		{"StatusKey", StatusKey, "infini.cloud/status"},
		{"StepKey", StepKey, "infini.cloud/step"},
		{"StatusSuccess", StatusSuccess, "success"},
		{"StatusFailure", StatusFailure, "failure"},
		{"StatusInProgress", StatusInProgress, "in_progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %s, want %s", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestWebhookRetryConstants(t *testing.T) {
	if WebhookRetryMaxAttempts <= 0 {
		t.Error("WebhookRetryMaxAttempts should be positive")
	}
	if WebhookRetryInitialInterval <= 0 {
		t.Error("WebhookRetryInitialInterval should be positive")
	}
}

// Test helper types

type testObject struct {
	name string
}

type testObjectKind struct{}

func (t *testObjectKind) SetGroupVersionKind(gvk schema.GroupVersionKind) {}
func (t *testObjectKind) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "test.infini.cloud",
		Version: "v1",
		Kind:    "TestObject",
	}
}

func (t *testObject) GetObjectKind() schema.ObjectKind {
	return &testObjectKind{}
}

func (t *testObject) DeepCopyObject() runtime.Object {
	return &testObject{name: t.name}
}

func TestEventDeduplication(t *testing.T) {
	// Track number of requests received
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	recorder := NewWebhookEventRecorder(server.URL, "change-123", "cluster-test")
	webRecorder := recorder.(*WebhookEventRecorder)

	// Create a mock object
	obj := &testObject{name: "test-app"}

	annotations := map[string]string{
		PhaseKey:  "Reconcile",
		StatusKey: StatusSuccess,
		StepKey:   "SyncComponent",
	}

	// Send the same event multiple times
	for i := 0; i < 5; i++ {
		webRecorder.AnnotatedEventf(obj, annotations, "Normal", "ReconcileCompleted", "Test message")
	}

	// Wait for async sends to complete
	time.Sleep(500 * time.Millisecond)

	// Should only send once due to deduplication
	count := atomic.LoadInt32(&requestCount)
	if count != 1 {
		t.Errorf("Expected 1 request, got %d (deduplication failed)", count)
	}
}

func TestEventDeduplicationDifferentStatus(t *testing.T) {
	// Track number of requests received
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	recorder := NewWebhookEventRecorder(server.URL, "change-123", "cluster-test")
	webRecorder := recorder.(*WebhookEventRecorder)

	obj := &testObject{name: "test-app"}

	// Send events with different statuses - should NOT be deduplicated
	annotations1 := map[string]string{
		PhaseKey:  "Reconcile",
		StatusKey: StatusInProgress,
		StepKey:   "SyncComponent",
	}
	webRecorder.AnnotatedEventf(obj, annotations1, "Normal", "ReconcileStarted", "Starting")

	annotations2 := map[string]string{
		PhaseKey:  "Reconcile",
		StatusKey: StatusSuccess,
		StepKey:   "SyncComponent",
	}
	webRecorder.AnnotatedEventf(obj, annotations2, "Normal", "ReconcileCompleted", "Completed")

	// Wait for async sends to complete
	time.Sleep(500 * time.Millisecond)

	// Should send both events (different status)
	count := atomic.LoadInt32(&requestCount)
	if count != 2 {
		t.Errorf("Expected 2 requests for different statuses, got %d", count)
	}
}
