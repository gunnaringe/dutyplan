package planner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrTimeout = errors.New("the planner took too long and was stopped")

// Solver runs the planner: JSON in, JSON out.
type Solver interface {
	Solve(ctx context.Context, input []byte) ([]byte, error)
}

// ExecSolver starts the Python solver as a subprocess (`python -m dutyplan_solver --check`). The environment it
// gets is minimal: it is given the plan's data on stdin and nothing else.
type ExecSolver struct {
	Argv []string // e.g. ["/path/to/.venv/bin/python", "-m", "dutyplan_solver"]
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.buf.Len()+len(p) > l.limit {
		return 0, errors.New("the planner answered with too much data")
	}
	return l.buf.Write(p)
}

func (e ExecSolver) Solve(ctx context.Context, input []byte) ([]byte, error) {
	if len(e.Argv) == 0 {
		return nil, errors.New("no planner is set up")
	}
	cmd := exec.CommandContext(ctx, e.Argv[0], append(append([]string{}, e.Argv[1:]...), "--check")...)
	cmd.Stdin = bytes.NewReader(input)
	out := &limitedBuffer{limit: 20 << 20}
	errOut := &limitedBuffer{limit: 8 << 10}
	cmd.Stdout, cmd.Stderr = out, errOut
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "PYTHONIOENCODING=utf-8", "PYTHONDONTWRITEBYTECODE=1", "PYTHONUNBUFFERED=1"}
	cmd.WaitDelay = 5 * time.Second // do not hang on a child that keeps the pipes open
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ErrTimeout
	}
	if err != nil {
		tail := strings.TrimSpace(errOut.buf.String())
		if i := strings.LastIndex(tail, "\n"); i >= 0 {
			tail = tail[i+1:] // the last line of a Python traceback is the error itself
		}
		return nil, fmt.Errorf("the planner ended with an error (%v): %s", err, tail)
	}
	return out.buf.Bytes(), nil
}
