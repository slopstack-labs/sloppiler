package providers

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// Spinner shows a live animated progress indicator on stderr.
// It is safe to call OK or Fail exactly once after starting.
//
// When the attached stream cannot render ANSI escape sequences — a redirected
// run, NO_COLOR, or a Windows console that refused virtual-terminal processing —
// the animation is suppressed entirely and each step settles into a single
// static line instead of scrolling thousands of redraws into a log file.
type Spinner struct {
	progressLabel                string
	accumulatedInferenceTokens   atomic.Int64
	terminationSignalChannel     chan struct{}
	completionAcknowledgeChannel chan struct{}
}

// StartSpinner starts a new Spinner with the given progressLabel and returns it.
func StartSpinner(progressLabel string) *Spinner {
	spinnerProgressIndicatorInstance := &Spinner{
		progressLabel:                progressLabel,
		terminationSignalChannel:     make(chan struct{}),
		completionAcknowledgeChannel: make(chan struct{}),
	}
	if !animationsEnabled {
		// Keep the channel contract identical so OK and Fail stay unchanged.
		go func() {
			defer close(spinnerProgressIndicatorInstance.completionAcknowledgeChannel)
			<-spinnerProgressIndicatorInstance.terminationSignalChannel
		}()
		return spinnerProgressIndicatorInstance
	}
	go func() {
		defer close(spinnerProgressIndicatorInstance.completionAcknowledgeChannel)
		for iterationIndexVector := 0; ; iterationIndexVector++ {
			select {
			case <-spinnerProgressIndicatorInstance.terminationSignalChannel:
				return
			default:
				accumulatedTokenCount := spinnerProgressIndicatorInstance.accumulatedInferenceTokens.Load()
				if accumulatedTokenCount > 0 {
					fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s  %s%d tokens%s",
						ClearLine, Cyan,
						spinnerAnimationFrames[iterationIndexVector%len(spinnerAnimationFrames)],
						Reset, spinnerProgressIndicatorInstance.progressLabel,
						Dim, accumulatedTokenCount, Reset)
				} else {
					fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s",
						ClearLine, Cyan,
						spinnerAnimationFrames[iterationIndexVector%len(spinnerAnimationFrames)],
						Reset, spinnerProgressIndicatorInstance.progressLabel)
				}
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return spinnerProgressIndicatorInstance
}

// AddTokens increments the displayed token counter by the provided tokenDeltaIncrement.
func (spinnerProgressIndicatorContext *Spinner) AddTokens(tokenDeltaIncrement int64) {
	spinnerProgressIndicatorContext.accumulatedInferenceTokens.Add(tokenDeltaIncrement)
}

// SetTokens sets the displayed token counter to the provided absoluteTokenCount.
func (spinnerProgressIndicatorContext *Spinner) SetTokens(absoluteTokenCount int64) {
	spinnerProgressIndicatorContext.accumulatedInferenceTokens.Store(absoluteTokenCount)
}

// OK stops the spinner and prints a green success marker.
func (spinnerProgressIndicatorContext *Spinner) OK() {
	close(spinnerProgressIndicatorContext.terminationSignalChannel)
	<-spinnerProgressIndicatorContext.completionAcknowledgeChannel
	accumulatedTokenCount := spinnerProgressIndicatorContext.accumulatedInferenceTokens.Load()
	if accumulatedTokenCount > 0 {
		fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s  %s(%d tokens)%s\n",
			ClearLine, Green, GlyphOK, Reset, spinnerProgressIndicatorContext.progressLabel,
			Dim, accumulatedTokenCount, Reset)
	} else {
		fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s\n",
			ClearLine, Green, GlyphOK, Reset, spinnerProgressIndicatorContext.progressLabel)
	}
}

// Fail stops the spinner and prints a red failure marker.
func (spinnerProgressIndicatorContext *Spinner) Fail() {
	close(spinnerProgressIndicatorContext.terminationSignalChannel)
	<-spinnerProgressIndicatorContext.completionAcknowledgeChannel
	fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s\n",
		ClearLine, Red, GlyphFail, Reset, spinnerProgressIndicatorContext.progressLabel)
}
