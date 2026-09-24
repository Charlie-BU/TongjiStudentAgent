package a2a

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync"
	"time"

	event "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	protocol "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

type executor struct {
	service   ChatService
	sessionMu sync.Mutex
	mu        sync.Mutex
	sessions  map[string]string // verified owner + A2A context -> backend session
	running   map[protocol.TaskID]context.CancelFunc
}

func (e *executor) session(ctx context.Context, ec *a2asrv.ExecutorContext) (string, error) {
	owner, err := principal(ctx)
	if err != nil {
		return "", err
	}
	key := owner + "\x00" + ec.ContextID
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	if id, ok := e.sessions[key]; ok {
		return id, nil
	}
	// Unknown client-supplied contexts must never silently fork or bind another
	// user's backend session. Only SDK-generated contexts create new sessions.
	if ec.Message.ContextID != "" {
		return "", protocol.ErrTaskNotFound
	}
	if len(e.sessions) >= maxEntries {
		return "", errors.New("A2A context capacity reached")
	}
	s, err := e.service.CreateSession(ctx, "A2A conversation")
	if err != nil {
		return "", err
	}
	e.sessions[key] = s.ID
	return s.ID, nil
}

func (e *executor) Execute(ctx context.Context, ec *a2asrv.ExecutorContext) iter.Seq2[protocol.Event, error] {
	return func(yield func(protocol.Event, error) bool) {
		if ec.Message == nil || ec.Message.Role != protocol.MessageRoleUser {
			yield(nil, protocol.ErrInvalidParams)
			return
		}
		var parts []string
		for _, p := range ec.Message.Parts {
			if p == nil {
				yield(nil, protocol.ErrInvalidParams)
				return
			}
			text, ok := p.Content.(protocol.Text)
			if !ok {
				yield(nil, protocol.ErrUnsupportedContentType)
				return
			}
			parts = append(parts, string(text))
		}
		query := strings.TrimSpace(strings.Join(parts, "\n"))
		if query == "" {
			yield(nil, protocol.ErrInvalidParams)
			return
		}
		if err := e.service.ValidateModelTier("lite"); err != nil {
			yield(nil, protocol.ErrInternalError)
			return
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		id, err := e.session(ctx, ec)
		if err != nil {
			if errors.Is(err, protocol.ErrTaskNotFound) {
				yield(nil, protocol.ErrTaskNotFound)
			} else {
				yield(nil, protocol.ErrInternalError)
			}
			return
		}
		e.mu.Lock()
		e.running[ec.TaskID] = cancel
		e.mu.Unlock()
		defer func() { e.mu.Lock(); delete(e.running, ec.TaskID); e.mu.Unlock() }()
		if ec.StoredTask == nil && !yield(protocol.NewSubmittedTask(ec, ec.Message), nil) {
			return
		}
		if !yield(protocol.NewStatusUpdateEvent(ec, protocol.TaskStateWorking, nil), nil) {
			return
		}
		var artifactID protocol.ArtifactID
		var stopped, failed bool
		total := 0
		// Chat emits callbacks serially. Keep the bridge synchronous: each increment
		// is delivered before StreamSession completes, without a buffering goroutine.
		send := func(ev protocol.Event) {
			if stopped {
				return
			}
			if !yield(ev, nil) {
				stopped = true
				cancel()
			}
		}
		delta := func(text string) {
			if stopped || text == "" {
				return
			}
			total += len(text)
			if total > 1<<20 {
				failed = true
				cancel()
				return
			}
			var out *protocol.TaskArtifactUpdateEvent
			if artifactID == "" {
				out = protocol.NewArtifactEvent(ec, protocol.NewTextPart(text))
				artifactID = out.Artifact.ID
			} else {
				out = protocol.NewArtifactUpdateEvent(ec, artifactID, protocol.NewTextPart(text))
			}
			send(out)
		}
		response, runErr := e.service.StreamSession(ctx, id, query, func(ev event.Event) {
			if stopped || failed {
				return
			}
			switch ev.Type {
			case event.AssistantDelta:
				if data, ok := ev.Data.(event.AssistantDeltaData); ok {
					delta(data.Text)
				}
			case event.AgentStatus:
				if data, ok := ev.Data.(event.AgentStatusData); ok {
					send(protocol.NewStatusUpdateEvent(ec, protocol.TaskStateWorking, protocol.NewMessage(protocol.MessageRoleAgent, protocol.NewTextPart(data.Message))))
				}
			case event.RunFailed:
				failed = true
			}
			// Raw reasoning, tool arguments/results and upstream error details are not
			// part of the public A2A projection. Tools still execute in ChatService.
		}, "lite")
		if stopped {
			return
		}
		if runErr != nil || failed || ctx.Err() != nil {
			state := protocol.TaskStateFailed
			if errors.Is(ctx.Err(), context.Canceled) {
				state = protocol.TaskStateCanceled
			}
			send(protocol.NewStatusUpdateEvent(ec, state, protocol.NewMessage(protocol.MessageRoleAgent, protocol.NewTextPart("Agent execution did not complete"))))
			return
		}
		if artifactID == "" {
			delta(response)
		}
		if stopped {
			return
		}
		if failed {
			send(protocol.NewStatusUpdateEvent(ec, protocol.TaskStateFailed, nil))
			return
		}
		if artifactID != "" {
			end := protocol.NewArtifactUpdateEvent(ec, artifactID, protocol.NewTextPart(""))
			end.LastChunk = true
			send(end)
		}
		if !stopped {
			send(protocol.NewStatusUpdateEvent(ec, protocol.TaskStateCompleted, nil))
		}
	}
}

func (e *executor) Cancel(ctx context.Context, ec *a2asrv.ExecutorContext) iter.Seq2[protocol.Event, error] {
	return func(yield func(protocol.Event, error) bool) {
		e.mu.Lock()
		cancel := e.running[ec.TaskID]
		e.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		yield(protocol.NewStatusUpdateEvent(ec, protocol.TaskStateCanceled, nil), nil)
	}
}
