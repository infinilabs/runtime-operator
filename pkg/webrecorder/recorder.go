package webrecorder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// --- Constants for structured event annotations ---

const (
	// PhaseKey is the key used in annotations to specify the high-level phase of an operation (e.g., "ScaleDown", "Upgrade").
	PhaseKey = "infini.cloud/phase"
	// StatusKey is the key used in annotations to specify the current status of an event (e.g., "InProgress", "Success", "Failure").
	StatusKey = "infini.cloud/status"
	// StepKey is the key used in annotations to specify the specific step within a phase (e.g., "StepScaleDownSetReplicas").
	StepKey = "infini.cloud/step"
)

// --- Constants for event status values ---

const (
	// StatusSuccess indicates that an operation or step completed successfully.
	StatusSuccess = "success"
	// StatusFailure indicates that an operation or step failed.
	StatusFailure = "failure"
	// StatusInProgress indicates that an operation or step is currently underway.
	StatusInProgress = "in_progress"
)

// --- Constants for Webhook sender configuration ---
const (
	// WebhookRetryMaxAttempts is the maximum number of times to retry sending a webhook event upon failure.
	WebhookRetryMaxAttempts = 3
	// WebhookRetryInitialInterval is the base duration to wait before the first retry.
	// Subsequent retries will use exponential backoff.
	WebhookRetryInitialInterval = 10 * time.Second
)

// ResourceChange tracks resource changes (CPU, memory, disk, replicas)
type ResourceChange struct {
	// CPUBefore is the CPU value before the change
	CPUBefore string `json:"cpu_before,omitempty"`
	// CPUAfter is the CPU value after the change
	CPUAfter string `json:"cpu_after,omitempty"`
	// MemoryBefore is the memory value before the change
	MemoryBefore string `json:"memory_before,omitempty"`
	// MemoryAfter is the memory value after the change
	MemoryAfter string `json:"memory_after,omitempty"`
	// DiskBefore is the disk/storage value before the change
	DiskBefore string `json:"disk_before,omitempty"`
	// DiskAfter is the disk/storage value after the change
	DiskAfter string `json:"disk_after,omitempty"`
	// ReplicasBefore is the replica count before the change
	ReplicasBefore *int32 `json:"replicas_before,omitempty"`
	// ReplicasAfter is the replica count after the change
	ReplicasAfter *int32 `json:"replicas_after,omitempty"`
}

// WebhookEvent defines the structured payload sent to an external webrecorder endpoint.
// This structure allows for easy parsing, indexing, and alerting by downstream systems.
type WebhookEvent struct {
	// ChangeID is a unique identifier for the entire reconciliation process or operation,
	// allowing for easy correlation of all related events.
	ChangeID string `json:"change_id"`
	// ClusterID identifies the cluster from which the event originated, crucial in multi-cluster environments.
	ClusterID string `json:"cluster_id"`
	// Phase represents the high-level operation being performed (e.g., "ScaleDown", "Upgrade").
	Phase string `json:"phase"`
	// Level corresponds to the Kubernetes event type ("Normal", "Warning"), providing severity context.
	Level string `json:"level"`
	// Message is the human-readable description of the event.
	Message string `json:"message"`
	// Timestamp is the UTC timestamp of when the event was generated, in RFC3339 format.
	Timestamp string `json:"timestamp"`
	// Payload can be used to attach additional, arbitrary key-value data for more context.
	Payload map[string]string `json:"payload,omitempty"`
	// Status provides a clear, machine-readable status of the event.
	Status string `json:"status"`
	// Step indicates the specific, granular step within the phase.
	Step string `json:"step"`
	// ResourceChange tracks changes in CPU, memory, disk, and replicas
	ResourceChange *ResourceChange `json:"resource_change,omitempty"`
}

// WebhookEventRecorder is a custom implementation of the record.EventRecorder interface.
// It acts as a decorator, wrapping a standard Kubernetes event recorder. In addition to
// recording events to the Kubernetes API, it also marshals a structured version of the
// event and sends it to a configured webrecorder URL.
type WebhookEventRecorder struct {
	// recorder is the underlying Kubernetes event recorder that this wraps.
	recorder record.EventRecorder
	// httpClient is used to send events to the webrecorder.
	httpClient *http.Client
	// webhookURL is the destination endpoint for the event payloads.
	webhookURL string
	// eventID is a static identifier for the source of this event stream (e.g., a pod name).
	eventID string
	// clusterID is a static identifier for the cluster.
	clusterID string
	// logger is used for internal logging of the recorder itself.
	logger logr.Logger
}

// NewWebhookEventRecorder creates a new WebhookEventRecorder.
// It requires a webhookURL, identifiers for the event source (eventID) and cluster (clusterID),
// and an existing recorder (typically from the controller-runtime manager) to wrap.
func NewWebhookEventRecorder(webhookURL, eventID, clusterID string) record.EventRecorder {
	return &WebhookEventRecorder{
		webhookURL: webhookURL,
		eventID:    eventID,
		clusterID:  clusterID,
		logger:     log.Log.WithName("Web Event Recorder"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Event passes a simple event through to the underlying recorder.
// It does not trigger a webrecorder.
func (r *WebhookEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	if r == nil {
		return
	}
	if r.recorder != nil {
		r.recorder.Event(object, eventtype, reason, message)
	}
}

// Eventf passes a formatted event through to the underlying recorder.
// It does not trigger a webrecorder.
func (r *WebhookEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	if r == nil {
		return
	}
	if r.recorder != nil {
		r.recorder.Eventf(object, eventtype, reason, messageFmt, args...)
	}
}

// AnnotatedEventf is the primary method for this recorder.
// It sends the event to the underlying recorder AND asynchronously sends a structured
// version of the event to the configured webrecorder.
func (r *WebhookEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	r.AnnotatedEventfWithResourceChange(object, annotations, nil, eventtype, reason, messageFmt, args...)
}

// AnnotatedEventfWithResourceChange is similar to AnnotatedEventf but includes resource change tracking.
// It sends the event to the underlying recorder AND asynchronously sends a structured
// version of the event with resource change information to the configured webrecorder.
func (r *WebhookEventRecorder) AnnotatedEventfWithResourceChange(object runtime.Object, annotations map[string]string, resourceChange *ResourceChange, eventtype, reason, messageFmt string, args ...interface{}) {
	if r == nil {
		return
	}

	// Send to underlying recorder if available
	if r.recorder != nil {
		r.recorder.Eventf(object, eventtype, reason, messageFmt, args...)
	}

	// If no webrecorder URL is configured, do nothing further.
	if r.webhookURL == "" {
		return
	}

	// Construct the structured event payload.
	eventData := &WebhookEvent{
		ChangeID:       r.eventID,
		ClusterID:      r.clusterID,
		Phase:          annotations[PhaseKey],
		Level:          eventtype,
		Message:        fmt.Sprintf(messageFmt, args...),
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		Payload:        map[string]string{"reason": reason},
		Status:         annotations[StatusKey],
		Step:           annotations[StepKey],
		ResourceChange: resourceChange,
	}

	// Send the event asynchronously to avoid blocking the reconciler.
	go r.sendEvent(eventData)
}

// sendEvent marshals the event data and posts it to the webhook URL.
// It implements a retry mechanism with exponential backoff for transient failures.
func (r *WebhookEventRecorder) sendEvent(data *WebhookEvent) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		r.logger.Error(err, "Failed to marshal webhook event data, the event will not be sent")
		return
	}

	for attempt := 0; attempt < WebhookRetryMaxAttempts; attempt++ {
		// For the first attempt, send immediately. For subsequent attempts, wait.
		if attempt > 0 {
			// Calculate exponential backoff: 2s, 4s, 8s
			backoffDuration := WebhookRetryInitialInterval * time.Duration(math.Pow(2, float64(attempt-1)))
			r.logger.Info("Webhook send failed. Retrying...",
				"url", r.webhookURL,
				"attempt", fmt.Sprintf("%d/%d", attempt+1, WebhookRetryMaxAttempts),
				"retry_after", backoffDuration.String())
			time.Sleep(backoffDuration)
		}

		// Use a context with a timeout for each individual request attempt.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", r.webhookURL, bytes.NewBuffer(jsonData))
		if err != nil {
			// This is a non-retriable error (failed to create the request itself).
			r.logger.Error(err, "Failed to create webhook request, the event will not be sent")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.httpClient.Do(req)
		if err != nil {
			r.logger.Error(err, "Failed to send webhook event", "url", r.webhookURL)
			// 快速失败：如果是连接拒绝或EOF，不再重试
			if isNetworkError(err) {
				r.logger.Info("Network error detected, skipping remaining retries", "error", err.Error())
				return
			}
			continue // Proceed to the next retry attempt
		}
		defer resp.Body.Close()

		// Read the response body
		bodyBytes, err := io.ReadAll(resp.Body)
		respBody := string(bodyBytes)

		// On successful send, check the status code.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			r.logger.V(1).Info("Successfully sent event to webhook", "url", r.webhookURL, "status", resp.Status)
			return // Success, exit the function.
		}

		// If the status code indicates an error, treat it as a failure.
		r.logger.Error(nil, "Webhook endpoint returned error status",
			"url", r.webhookURL,
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"response_body", respBody)
	}

	// If the loop completes, all retries have failed.
	r.logger.Error(nil, "Failed to send webhook event after all retries, dropping the event.",
		"url", r.webhookURL,
		"max_retries", WebhookRetryMaxAttempts)
}

// isNetworkError checks if the error is a network-related error that should not be retried
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common non-retriable network errors
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable")
}
