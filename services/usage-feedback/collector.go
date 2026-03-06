package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// UsageEventType classifies the kind of usage feedback signal.
type UsageEventType string

const (
	EventRating      UsageEventType = "rating"
	EventRetry       UsageEventType = "retry"
	EventSessionEnd  UsageEventType = "session_end"
	EventFollowUp    UsageEventType = "follow_up"
)

// UsageEvent represents a single usage feedback event from the API gateway.
type UsageEvent struct {
	EventID   string         `json:"event_id"`
	Type      UsageEventType `json:"type"`
	Timestamp time.Time      `json:"timestamp"`

	// For rating events
	Rating int `json:"rating,omitempty"` // +1 (thumbs up) or -1 (thumbs down)

	// For retry events
	OriginalQuery string `json:"original_query,omitempty"`

	// For session events
	SessionDuration float64 `json:"session_duration_sec,omitempty"`
	QueryCount      int     `json:"query_count,omitempty"`

	// For follow-up events
	FollowUpText string `json:"follow_up_text,omitempty"`

	// Common: the query/response context
	Query       string `json:"query"`
	Domain      string `json:"domain,omitempty"`
	AdapterID   string `json:"adapter_id,omitempty"`
}

// Collector parses API gateway usage event logs.
type Collector struct{}

// NewCollector creates a new Collector.
func NewCollector() *Collector {
	return &Collector{}
}

// ParseEventLog reads a JSONL file of usage events and returns parsed events.
func (c *Collector) ParseEventLog(path string) ([]UsageEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer f.Close()

	var events []UsageEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024) // 10MB buffer

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var event UsageEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("line %d: unmarshal event: %w", lineNum, err)
		}

		if !c.validateEvent(&event) {
			continue
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan event log: %w", err)
	}

	return events, nil
}

// validateEvent checks that an event has the required fields for its type.
func (c *Collector) validateEvent(e *UsageEvent) bool {
	if e.Query == "" {
		return false
	}

	switch e.Type {
	case EventRating:
		return e.Rating == 1 || e.Rating == -1
	case EventRetry:
		return e.OriginalQuery != ""
	case EventSessionEnd:
		return e.SessionDuration > 0
	case EventFollowUp:
		return e.FollowUpText != ""
	default:
		return false
	}
}

// ClassifyFollowUp determines if a follow-up message is positive or negative.
// Returns a signal in [-1.0, 1.0].
func (c *Collector) ClassifyFollowUp(text string) float64 {
	lower := strings.ToLower(text)

	negativePatterns := []string{
		"that's wrong", "try again", "incorrect", "not what i asked",
		"wrong answer", "that doesn't work", "no that's not",
		"please fix", "that's not right", "you're wrong",
	}
	positivePatterns := []string{
		"thanks", "thank you", "perfect", "great", "exactly",
		"that works", "helpful", "awesome", "good answer",
		"nice", "well done",
	}

	for _, p := range negativePatterns {
		if strings.Contains(lower, p) {
			return -1.0
		}
	}

	for _, p := range positivePatterns {
		if strings.Contains(lower, p) {
			return 1.0
		}
	}

	return 0.0 // neutral / unclassifiable
}

// EventToSignal converts a usage event to a raw signal value in [-1.0, 1.0].
func (c *Collector) EventToSignal(event *UsageEvent) float64 {
	switch event.Type {
	case EventRating:
		return float64(event.Rating) // +1 or -1

	case EventRetry:
		return -0.5 // retry = mild negative signal

	case EventSessionEnd:
		// Longer productive sessions = positive signal
		// Short sessions (<30s) = negative, medium (30s-300s) = neutral, long (>300s) = positive
		if event.SessionDuration < 30 {
			return -0.3
		}
		if event.SessionDuration > 300 && event.QueryCount >= 3 {
			return 0.5
		}
		return 0.0

	case EventFollowUp:
		return c.ClassifyFollowUp(event.FollowUpText)

	default:
		return 0.0
	}
}
