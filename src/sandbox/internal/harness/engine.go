package harness

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/audit"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/jobstore"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/llm"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/toolproxy"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/verdict"
)

// Engine runs the agentic loop for one job at a time. It is
// stateless across jobs — Run is safe to call concurrently from
// multiple workers.
type Engine struct {
	registry *Registry
	llm      llm.Client
	proxy    *toolproxy.Proxy
	audit    audit.Logger
}

// NewEngine wires the engine. The constructor is dependency
// injection, no globals.
func NewEngine(reg *Registry, model llm.Client, proxy *toolproxy.Proxy, logger audit.Logger) *Engine {
	return &Engine{
		registry: reg,
		llm:      model,
		proxy:    proxy,
		audit:    logger,
	}
}

// Run executes one job to completion. The returned verdict is
// guaranteed to validate against the verdict schema; the cost is the
// total spent across LLM and tool calls during the run.
func (e *Engine) Run(ctx context.Context, job *jobstore.Job) (*verdict.Verdict, int, error) {
	tmpl, err := e.registry.Lookup(job.Brief.Objective)
	if err != nil {
		return nil, 0, err
	}

	if err := tmpl.Validate(&job.Brief); err != nil {
		v := &verdict.Verdict{
			Assessment:         verdict.AssessmentInsufficientData,
			Confidence:         verdict.ConfidenceLow,
			SourcesConsulted:   []string{},
			Agreement:          verdict.AgreementSummary{},
			RecommendedAction:  verdict.ActionNone,
			InsufficientReason: verdict.ReasonSubjectMissing,
		}
		return v, 0, nil
	}

	budget := job.Brief.BudgetCents
	if budget <= 0 {
		budget = tmpl.DefaultBudgetCents
	}

	subjectID, _ := job.Brief.SubjectField(tmpl.SubjectIDField)
	allowed := tmpl.AllowedToolSet()

	transcript := []llm.Message{
		{Role: llm.RoleSystem, Content: tmpl.SystemPrompt},
		{Role: llm.RoleUser, Content: "subject:" + subjectID},
	}

	cost := 0

	for turn := 1; turn <= tmpl.TurnLimit; turn++ {
		if cost >= budget {
			return budgetExhausted(cost, turn-1), cost, nil
		}

		reply, err := e.llm.Turn(ctx, transcript)
		if err != nil {
			return nil, cost, fmt.Errorf("llm turn %d: %w", turn, err)
		}
		cost += reply.CostCents

		e.audit.Append(audit.Record{
			Kind:       audit.EventLLMTurn,
			JobID:      job.ID,
			TurnNumber: turn,
			CostCents:  reply.CostCents,
		})

		switch reply.Kind {
		case llm.ReplyVerdict:
			v := reply.Verdict
			if v == nil {
				return nil, cost, errors.New("harness: model emitted empty verdict")
			}
			if err := v.Validate(); err != nil {
				return nil, cost, fmt.Errorf("harness: verdict invalid: %w", err)
			}
			v.CostCents = cost
			v.TurnsUsed = turn
			return v, cost, nil

		case llm.ReplyToolUse:
			if !allowed[reply.ToolID] {
				return noMatchingSources(cost, turn), cost, nil
			}
			result, err := e.proxy.Call(ctx, toolproxy.Call{
				JobID:        job.ID,
				ToolID:       reply.ToolID,
				SubjectID:    subjectID,
				Parameters:   reply.ToolParams,
				AllowedTools: allowed,
			})
			if err != nil {
				log.Printf("harness: tool call %s failed: %v", reply.ToolID, err)
				return noMatchingSources(cost, turn), cost, nil
			}
			cost += result.CostCents
			transcript = append(transcript, llm.Message{
				Role:       llm.RoleTool,
				ToolID:     reply.ToolID,
				ToolResult: result.Records,
			})

		case llm.ReplyText:
			transcript = append(transcript, llm.Message{
				Role:    llm.RoleAssistant,
				Content: reply.Text,
			})

		default:
			return nil, cost, fmt.Errorf("harness: unknown reply kind %q", reply.Kind)
		}
	}

	return turnLimitExceeded(cost, tmpl.TurnLimit), cost, nil
}

func budgetExhausted(cost, turns int) *verdict.Verdict {
	return &verdict.Verdict{
		Assessment:         verdict.AssessmentInsufficientData,
		Confidence:         verdict.ConfidenceLow,
		SourcesConsulted:   []string{},
		Agreement:          verdict.AgreementSummary{},
		RecommendedAction:  verdict.ActionResubmitMoreBudget,
		InsufficientReason: verdict.ReasonBudgetExhausted,
		CostCents:          cost,
		TurnsUsed:          turns,
	}
}

func noMatchingSources(cost, turns int) *verdict.Verdict {
	return &verdict.Verdict{
		Assessment:         verdict.AssessmentInsufficientData,
		Confidence:         verdict.ConfidenceLow,
		SourcesConsulted:   []string{},
		Agreement:          verdict.AgreementSummary{},
		RecommendedAction:  verdict.ActionNone,
		InsufficientReason: verdict.ReasonNoMatchingSources,
		CostCents:          cost,
		TurnsUsed:          turns,
	}
}

func turnLimitExceeded(cost, turns int) *verdict.Verdict {
	return &verdict.Verdict{
		Assessment:         verdict.AssessmentInsufficientData,
		Confidence:         verdict.ConfidenceLow,
		SourcesConsulted:   []string{},
		Agreement:          verdict.AgreementSummary{},
		RecommendedAction:  verdict.ActionResubmitMoreBudget,
		InsufficientReason: verdict.ReasonTurnLimitExceeded,
		CostCents:          cost,
		TurnsUsed:          turns,
	}
}

// Worker pops jobs from the queue and runs them through the engine.
type Worker struct {
	id     int
	store  jobstore.Store
	queue  jobstore.Queue
	engine *Engine
	audit  audit.Logger
}

// NewWorker builds a worker. Construct one per goroutine.
func NewWorker(id int, store jobstore.Store, queue jobstore.Queue, engine *Engine, logger audit.Logger) *Worker {
	return &Worker{id: id, store: store, queue: queue, engine: engine, audit: logger}
}

// Start blocks until ctx is cancelled, popping and running jobs as
// they arrive.
func (w *Worker) Start(ctx context.Context) {
	for {
		jobID, err := w.queue.Pop(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			log.Printf("worker %d: queue pop: %v", w.id, err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		w.handle(ctx, jobID)
	}
}

func (w *Worker) handle(ctx context.Context, jobID string) {
	job, err := w.store.Get(ctx, jobID)
	if err != nil {
		log.Printf("worker %d: get %s: %v", w.id, jobID, err)
		return
	}

	if err := w.store.UpdateState(ctx, jobID, jobstore.StateRunning); err != nil {
		log.Printf("worker %d: mark running %s: %v", w.id, jobID, err)
		return
	}
	w.audit.Append(audit.Record{Kind: audit.EventJobStarted, JobID: jobID})

	v, cost, err := w.engine.Run(ctx, job)
	if err != nil {
		_ = w.store.Fail(ctx, jobID, err.Error())
		w.audit.Append(audit.Record{Kind: audit.EventJobFailed, JobID: jobID, Message: err.Error()})
		return
	}

	if err := w.store.Complete(ctx, jobID, v, cost); err != nil {
		_ = w.store.Fail(ctx, jobID, err.Error())
		w.audit.Append(audit.Record{Kind: audit.EventJobFailed, JobID: jobID, Message: err.Error()})
		return
	}
	w.audit.Append(audit.Record{Kind: audit.EventVerdict, JobID: jobID, CostCents: cost})
	w.audit.Append(audit.Record{Kind: audit.EventJobCompleted, JobID: jobID, CostCents: cost})
}
