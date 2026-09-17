package providers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"
)

// Config holds shared runtime configuration for all Provider implementations.
type Config struct {
	// RequestExecutionTimeout is the maximum duration for a single streaming
	// request, covering DNS resolution, connection establishment, TLS handshake,
	// and full response body consumption. Zero disables the timeout entirely.
	RequestExecutionTimeout time.Duration
	// VerboseDebugLoggingEnabled enables diagnostic output to stderr showing
	// HTTP request targets, response status codes, and streaming progress milestones.
	VerboseDebugLoggingEnabled bool
}

// debugLogf writes a dim diagnostic line to stderr when VerboseDebugLoggingEnabled is true.
func (providerRuntimeConfig Config) debugLogf(diagnosticMessageFormat string, diagnosticMessageArgs ...any) {
	if providerRuntimeConfig.VerboseDebugLoggingEnabled {
		fmt.Fprintf(os.Stderr, "  "+Dim+"[debug] "+diagnosticMessageFormat+Reset+"\n", diagnosticMessageArgs...)
	}
}

// buildRequestExecutionContext returns a context scoped to the configured
// RequestExecutionTimeout along with its cancel function. When the timeout is
// zero, Background is returned with a no-op cancel so callers can always defer
// cancel unconditionally.
func buildRequestExecutionContext(providerRuntimeConfig Config) (context.Context, context.CancelFunc) {
	if providerRuntimeConfig.RequestExecutionTimeout > 0 {
		return context.WithTimeout(context.Background(), providerRuntimeConfig.RequestExecutionTimeout)
	}
	return context.Background(), func() {}
}

// sanitizeURLForDebugLogging strips query parameters from a URL before
// surfacing it in debug output to avoid leaking API keys embedded in query strings.
func sanitizeURLForDebugLogging(rawEndpointURL string) string {
	parsedEndpointURL, urlParseRemediationOpportunity := url.Parse(rawEndpointURL)
	if urlParseRemediationOpportunity != nil {
		return rawEndpointURL
	}
	parsedEndpointURL.RawQuery = ""
	return parsedEndpointURL.String()
}

// wrapStreamError returns a user-friendly error for stream failures, giving
// special messaging when the request context deadline was exceeded.
func wrapStreamError(requestContext context.Context, providerRuntimeConfig Config, scannerRemediationOpportunity error) error {
	if requestContext.Err() == context.DeadlineExceeded {
		return fmt.Errorf("request timed out after %v — use --timeout=0 to disable or increase --timeout",
			providerRuntimeConfig.RequestExecutionTimeout)
	}
	return fmt.Errorf("stream error: %w", scannerRemediationOpportunity)
}

// handleStreamCompletion resolves the final scanner error against accumulated
// content. If content was already received and the only error is a context
// deadline (i.e. the model finished generating right as the timer fired), the
// error is discarded and nil is returned so the caller can proceed normally.
// This prevents a false timeout failure when the stream logically completed
// before the deadline was observed by the scanner.
func handleStreamCompletion(requestExecutionContext context.Context, providerRuntimeConfig Config, scannerRemediationOpportunity error, accumulatedContentByteLength int) error {
	if scannerRemediationOpportunity == nil {
		return nil
	}
	if accumulatedContentByteLength > 0 && requestExecutionContext.Err() == context.DeadlineExceeded {
		providerRuntimeConfig.debugLogf("⚠ deadline fired after content received (%d bytes) — treating as complete", accumulatedContentByteLength)
		return nil
	}
	return wrapStreamError(requestExecutionContext, providerRuntimeConfig, scannerRemediationOpportunity)
}

// ── Stream progress coordination ──────────────────────────────────────────────

const defaultStreamProgressAdvancementTickInterval = 12 * time.Second

// StreamProgressCoordinator drives a sequence of labelled progress steps to
// completion as real streaming milestones occur. Steps advance through three
// anchor points — HTTP 200 received, first token received, and stream complete —
// with any remaining intermediate steps advancing on a time-based tick during
// active generation.
type StreamProgressCoordinator struct {
	progressStepLabels           []string
	activeProgressSpinner        *Spinner
	currentProgressStepIndex     int
	providerRuntimeConfig        Config
	firstTokenReceivedTimestamp  time.Time
	lastStepAdvancementTimestamp time.Time
	stepAdvancementTickInterval  time.Duration
}

// NewStreamProgressCoordinator creates a coordinator for the given steps and
// immediately starts a spinner for the first step.
func NewStreamProgressCoordinator(progressStepLabels []string, providerRuntimeConfig Config) *StreamProgressCoordinator {
	coordinatorInstance := &StreamProgressCoordinator{
		progressStepLabels:          progressStepLabels,
		providerRuntimeConfig:       providerRuntimeConfig,
		stepAdvancementTickInterval: defaultStreamProgressAdvancementTickInterval,
	}
	if len(progressStepLabels) > 0 {
		coordinatorInstance.activeProgressSpinner = StartSpinner(progressStepLabels[0])
	}
	return coordinatorInstance
}

func (streamProgressCoordinatorContext *StreamProgressCoordinator) advanceToNextProgressStep() {
	if streamProgressCoordinatorContext.activeProgressSpinner != nil {
		streamProgressCoordinatorContext.activeProgressSpinner.OK()
		streamProgressCoordinatorContext.activeProgressSpinner = nil
	}
	streamProgressCoordinatorContext.currentProgressStepIndex++
	if streamProgressCoordinatorContext.currentProgressStepIndex < len(streamProgressCoordinatorContext.progressStepLabels) {
		streamProgressCoordinatorContext.activeProgressSpinner = StartSpinner(
			streamProgressCoordinatorContext.progressStepLabels[streamProgressCoordinatorContext.currentProgressStepIndex],
		)
	}
}

// OnConnected is called when the HTTP 200 response header is received.
func (streamProgressCoordinatorContext *StreamProgressCoordinator) OnConnected() {
	if streamProgressCoordinatorContext.currentProgressStepIndex < len(streamProgressCoordinatorContext.progressStepLabels)-1 {
		streamProgressCoordinatorContext.advanceToNextProgressStep()
	}
}

// OnFirstToken is called when the first non-empty content token arrives.
func (streamProgressCoordinatorContext *StreamProgressCoordinator) OnFirstToken() {
	streamProgressCoordinatorContext.firstTokenReceivedTimestamp = time.Now()
	streamProgressCoordinatorContext.lastStepAdvancementTimestamp = time.Now()
	if streamProgressCoordinatorContext.currentProgressStepIndex < len(streamProgressCoordinatorContext.progressStepLabels)-1 {
		streamProgressCoordinatorContext.advanceToNextProgressStep()
	}
}

// OnTokens is called on every streaming chunk. It updates the spinner token
// display and advances intermediate steps on a time-based tick, always reserving
// the last step for OnComplete.
func (streamProgressCoordinatorContext *StreamProgressCoordinator) OnTokens(accumulatedInferenceTokenCount int64) {
	if streamProgressCoordinatorContext.activeProgressSpinner != nil && accumulatedInferenceTokenCount > 0 {
		streamProgressCoordinatorContext.activeProgressSpinner.SetTokens(accumulatedInferenceTokenCount)
	}
	if streamProgressCoordinatorContext.firstTokenReceivedTimestamp.IsZero() {
		return
	}
	if streamProgressCoordinatorContext.currentProgressStepIndex >= len(streamProgressCoordinatorContext.progressStepLabels)-1 {
		return
	}
	if time.Since(streamProgressCoordinatorContext.lastStepAdvancementTimestamp) >= streamProgressCoordinatorContext.stepAdvancementTickInterval {
		streamProgressCoordinatorContext.lastStepAdvancementTimestamp = time.Now()
		streamProgressCoordinatorContext.advanceToNextProgressStep()
	}
}

// OnComplete is called when the stream finishes. Sets the final token count and
// flushes all remaining steps to completion with a brief visual gap between each.
// Idempotent if already fully completed.
func (streamProgressCoordinatorContext *StreamProgressCoordinator) OnComplete(totalInferenceTokenCount int64) {
	if streamProgressCoordinatorContext.activeProgressSpinner != nil && totalInferenceTokenCount > 0 {
		streamProgressCoordinatorContext.activeProgressSpinner.SetTokens(totalInferenceTokenCount)
	}
	for streamProgressCoordinatorContext.currentProgressStepIndex < len(streamProgressCoordinatorContext.progressStepLabels) {
		streamProgressCoordinatorContext.advanceToNextProgressStep()
		if streamProgressCoordinatorContext.currentProgressStepIndex < len(streamProgressCoordinatorContext.progressStepLabels) {
			time.Sleep(150 * time.Millisecond)
		}
	}
}

// OnFail fails the currently active progress step and exhausts all remaining
// steps so no stale spinners remain. Idempotent if already completed.
func (streamProgressCoordinatorContext *StreamProgressCoordinator) OnFail() {
	if streamProgressCoordinatorContext.currentProgressStepIndex >= len(streamProgressCoordinatorContext.progressStepLabels) {
		return
	}
	if streamProgressCoordinatorContext.activeProgressSpinner != nil {
		streamProgressCoordinatorContext.activeProgressSpinner.Fail()
		streamProgressCoordinatorContext.activeProgressSpinner = nil
	}
	streamProgressCoordinatorContext.currentProgressStepIndex = len(streamProgressCoordinatorContext.progressStepLabels)
}
