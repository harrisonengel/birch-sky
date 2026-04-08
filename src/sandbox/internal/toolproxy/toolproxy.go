// Package toolproxy is the only path the harness uses to reach data.
//
// The harness has no other network egress. Every tool call goes
// through `Proxy.Call`, which authorizes the call against the active
// template, routes it to the matching `Source`, meters cost, and
// appends an audit record. Adding a data source means adding a
// `Source` implementation and registering it in `NewProxy`.
//
// The MVP ships one source: `provider_directory`, which returns a
// single hello-world healthcare provider record. The contract is
// shaped to fit any future verification source.
package toolproxy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/audit"
)

// Call is one tool invocation issued by the harness.
type Call struct {
	JobID       string
	ToolID      string
	SubjectID   string         // the entity being investigated, scoped per session
	Parameters  map[string]any // query shape — never raw seller data
	AllowedTools map[string]bool
}

// Result is the proxy's response. Records are normalized field maps
// the harness uses to drive its categorical verdict — never raw
// documents.
type Result struct {
	ToolID    string           `json:"tool_id"`
	Records   []map[string]any `json:"records"`
	CostCents int              `json:"cost_cents"`
}

// Source is one data source adapter.
type Source interface {
	ID() string
	// CostCents is the flat per-call price for the MVP.
	CostCents() int
	// Query returns normalized records for a subject. Sources never
	// return raw documents — only the fields the proxy has decided
	// the harness needs.
	Query(ctx context.Context, subjectID string, params map[string]any) ([]map[string]any, error)
}

// Proxy is the single door between the harness and the data sources.
type Proxy struct {
	mu      sync.RWMutex
	sources map[string]Source
	audit   audit.Logger
}

// NewProxy constructs a Proxy with the supplied audit logger and a
// pre-registered set of sources.
func NewProxy(logger audit.Logger, sources ...Source) *Proxy {
	p := &Proxy{
		sources: make(map[string]Source, len(sources)),
		audit:   logger,
	}
	for _, s := range sources {
		p.sources[s.ID()] = s
	}
	return p
}

// Register adds (or replaces) a source after construction. Useful for
// tests and for the future seller-onboarding flow that wires new
// sources at runtime.
func (p *Proxy) Register(s Source) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sources[s.ID()] = s
}

// Sources returns the IDs of all currently registered sources.
func (p *Proxy) Sources() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, 0, len(p.sources))
	for id := range p.sources {
		out = append(out, id)
	}
	return out
}

// ErrToolNotAuthorized is returned when the active session's template
// did not declare the requested tool in its allow-list.
var ErrToolNotAuthorized = errors.New("toolproxy: tool not authorized for this session")

// ErrToolNotFound is returned when the requested tool ID has no
// matching source.
var ErrToolNotFound = errors.New("toolproxy: tool not found")

// Call routes one tool invocation. The harness owns the call site;
// the proxy owns the policy.
func (p *Proxy) Call(ctx context.Context, c Call) (*Result, error) {
	if c.AllowedTools != nil && !c.AllowedTools[c.ToolID] {
		p.audit.Append(audit.Record{
			Kind:    audit.EventToolCall,
			JobID:   c.JobID,
			ToolID:  c.ToolID,
			Message: "denied: tool not authorized",
		})
		return nil, fmt.Errorf("%w: %s", ErrToolNotAuthorized, c.ToolID)
	}

	p.mu.RLock()
	source, ok := p.sources[c.ToolID]
	p.mu.RUnlock()
	if !ok {
		p.audit.Append(audit.Record{
			Kind:    audit.EventToolCall,
			JobID:   c.JobID,
			ToolID:  c.ToolID,
			Message: "denied: tool not found",
		})
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, c.ToolID)
	}

	start := time.Now()
	records, err := source.Query(ctx, c.SubjectID, c.Parameters)
	latency := time.Since(start)
	if err != nil {
		p.audit.Append(audit.Record{
			Kind:      audit.EventToolCall,
			JobID:     c.JobID,
			ToolID:    c.ToolID,
			LatencyMS: int(latency.Milliseconds()),
			Message:   "error: " + err.Error(),
		})
		return nil, err
	}

	cost := source.CostCents()
	p.audit.Append(audit.Record{
		Kind:         audit.EventToolCall,
		JobID:        c.JobID,
		ToolID:       c.ToolID,
		QueryShape:   shapeOf(c.Parameters),
		ResponseSize: len(records),
		CostCents:    cost,
		LatencyMS:    int(latency.Milliseconds()),
	})

	return &Result{ToolID: c.ToolID, Records: records, CostCents: cost}, nil
}

// shapeOf returns a stable description of the parameter keys without
// leaking values into the audit log.
func shapeOf(params map[string]any) string {
	if len(params) == 0 {
		return "{}"
	}
	out := "{"
	first := true
	for k := range params {
		if !first {
			out += ","
		}
		out += k
		first = false
	}
	out += "}"
	return out
}
