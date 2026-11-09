package planner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gunnaringe/dutyplan/internal/store"
)

var (
	ErrBusy          = errors.New("a planning run is already in progress; wait for it to finish")
	ErrUnavailable   = errors.New("the planner is not set up on this server")
	ErrNothingToPlan = store.Invalid("nothing_to_plan", nil, "nothing to plan: add events, day types with tasks and people first")
)

const (
	DefaultTimeLimit = 20
	MinTimeLimit     = 5
	MaxTimeLimit     = 120
	grace            = 30 * time.Second // Python start-up and presolve may run a little past the solver's own time limit
)

// Service starts runs and keeps at most one going at a time (CP-SAT uses every core).
type Service struct {
	st      *store.Store
	solver  Solver
	log     *slog.Logger
	workers int

	timeoutOverride time.Duration // tests only: replaces time limit + grace

	mu      sync.Mutex
	running bool
	wg      sync.WaitGroup
}

// New returns a service; solver may be nil, in which case planning reports ErrUnavailable.
func New(st *store.Store, solver Solver, workers int, log *slog.Logger) *Service {
	if workers < 1 {
		workers = 4
	}
	return &Service{st: st, solver: solver, workers: workers, log: log}
}

func (s *Service) Available() bool { return s.solver != nil }

// Wait blocks until the run in progress (if any) has finished. For tests and shutdown.
func (s *Service) Wait() { s.wg.Wait() }

// Start begins a run in the background and returns its id.
func (s *Service) Start(ctx context.Context, orgSlug, planSlug, actor string, p Params) (int64, error) {
	if s.solver == nil {
		return 0, ErrUnavailable
	}
	if p.TimeLimit == 0 {
		p.TimeLimit = DefaultTimeLimit
	}
	if p.TimeLimit < MinTimeLimit || p.TimeLimit > MaxTimeLimit {
		return 0, store.Invalid("time_limit", map[string]any{"min": MinTimeLimit, "max": MaxTimeLimit}, "the time limit must be %d-%d seconds", MinTimeLimit, MaxTimeLimit)
	}
	data, err := s.st.GetPlanningData(ctx, orgSlug, planSlug)
	if err != nil {
		return 0, err
	}
	if !HasWork(data) {
		return 0, ErrNothingToPlan
	}
	if p.Seed == 0 {
		if p.Seed, err = s.st.NextSeed(ctx, orgSlug, planSlug); err != nil {
			return 0, err
		}
	}
	p.Workers = s.workers

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return 0, ErrBusy
	}
	s.running = true
	s.mu.Unlock()
	release := func() { s.mu.Lock(); s.running = false; s.mu.Unlock() }

	input, hash, snap, err := BuildInput(data, p)
	if err != nil {
		release()
		return 0, err
	}
	snapJSON, _ := json.Marshal(snap)
	id, err := s.st.CreateRun(ctx, orgSlug, planSlug, actor, p.Seed, p.TimeLimit, hash, input, snapJSON)
	if err != nil {
		release()
		return 0, err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer release()
		s.execute(id, input, p.TimeLimit)
	}()
	return id, nil
}

func (s *Service) execute(id int64, input []byte, timeLimit int) {
	finish := func(out []byte, msg string) {
		if err := s.st.FinishRun(context.Background(), id, out, msg); err != nil {
			s.log.Error("could not store the run result", "run", id, "err", err)
		}
	}
	defer func() {
		if v := recover(); v != nil {
			s.log.Error("planner panic", "run", id, "value", v, "stack", string(debug.Stack()))
			finish(nil, "internal error while planning")
		}
	}()
	limit := time.Duration(timeLimit)*time.Second + grace
	if s.timeoutOverride > 0 {
		limit = s.timeoutOverride
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	out, err := s.solver.Solve(ctx, input)
	if err != nil {
		s.log.Warn("planning failed", "run", id, "err", err)
		finish(nil, err.Error())
		return
	}
	var head struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(out, &head) != nil || head.Status == "" {
		finish(nil, "the planner gave an answer that could not be read")
		return
	}
	if head.Status != "OPTIMAL" && head.Status != "FEASIBLE" {
		finish(nil, "the planner found no plan ("+head.Status+")")
		return
	}
	finish(out, "")
}
