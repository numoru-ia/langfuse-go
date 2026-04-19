// Package langfuse provides a minimal batching client for Langfuse v3.
package langfuse

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Config for a Langfuse client.
type Config struct {
	BaseURL   string
	PublicKey string
	SecretKey string
	// FlushInterval controls async flushing; defaults to 3s.
	FlushInterval time.Duration
	HTTPClient    *http.Client
}

// Client is a Langfuse v3 API wrapper.
type Client struct {
	cfg    Config
	mu     sync.Mutex
	buffer []ingestEvent
	stop   chan struct{}
}

type ingestEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Body      map[string]any `json:"body"`
}

// TraceInput configures a new trace.
type TraceInput struct {
	Name      string
	SessionID string
	UserID    string
	Metadata  map[string]any
	Tags      []string
}

// GenerationInput describes an LLM call within a trace.
type GenerationInput struct {
	Name   string
	Model  string
	Input  any
	Output any
	Usage  *Usage
}

type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// New creates a client.
func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 3 * time.Second
	}
	c := &Client{cfg: cfg, stop: make(chan struct{})}
	go c.flushLoop()
	return c
}

// Trace starts a new trace handle.
func (c *Client) Trace(in *TraceInput) *Trace {
	id := newID()
	c.append(ingestEvent{
		ID:        newID(),
		Type:      "trace-create",
		Timestamp: time.Now(),
		Body: map[string]any{
			"id":        id,
			"name":      in.Name,
			"sessionId": in.SessionID,
			"userId":    in.UserID,
			"metadata":  in.Metadata,
			"tags":      in.Tags,
		},
	})
	return &Trace{client: c, id: id}
}

// Trace handle.
type Trace struct {
	client *Client
	id     string
}

// Generation records an LLM call within this trace.
func (t *Trace) Generation(in *GenerationInput) {
	t.client.append(ingestEvent{
		ID:        newID(),
		Type:      "generation-create",
		Timestamp: time.Now(),
		Body: map[string]any{
			"id":      newID(),
			"traceId": t.id,
			"name":    in.Name,
			"model":   in.Model,
			"input":   in.Input,
			"output":  in.Output,
			"usage":   in.Usage,
		},
	})
}

// Flush sends the buffer synchronously. Call before process exit.
func (c *Client) Flush(ctx context.Context) error {
	c.mu.Lock()
	batch := c.buffer
	c.buffer = nil
	c.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	return c.send(ctx, batch)
}

// Close stops background flushing.
func (c *Client) Close(ctx context.Context) error {
	close(c.stop)
	return c.Flush(ctx)
}

func (c *Client) append(e ingestEvent) {
	c.mu.Lock()
	c.buffer = append(c.buffer, e)
	c.mu.Unlock()
}

func (c *Client) flushLoop() {
	t := time.NewTicker(c.cfg.FlushInterval)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			_ = c.Flush(context.Background())
		}
	}
}

func (c *Client) send(ctx context.Context, events []ingestEvent) error {
	body, err := json.Marshal(map[string]any{"batch": events})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/api/public/ingestion", bytes.NewReader(body))
	if err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.cfg.PublicKey + ":" + c.cfg.SecretKey))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("langfuse ingestion status %d", res.StatusCode)
	}
	return nil
}

func newID() string {
	var b [16]byte
	_, _ = timeBasedRand(&b)
	return fmt.Sprintf("%x", b)
}

func timeBasedRand(b *[16]byte) (int, error) {
	now := time.Now().UnixNano()
	for i := 0; i < 8; i++ {
		b[i] = byte(now >> (i * 8))
	}
	for i := 8; i < 16; i++ {
		b[i] = byte(time.Now().Nanosecond() >> (i % 4 * 8))
	}
	return 16, nil
}
