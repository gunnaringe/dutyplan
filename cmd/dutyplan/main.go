// Command dutyplan serves the duty-planning app: one binary, SQLite, embedded frontend. No login.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gunnaringe/dutyplan/internal/api"
	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
	"github.com/gunnaringe/dutyplan/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on (port 0 picks a free one)")
	dbPath := flag.String("db", "dutyplan.db", "SQLite database file (created if missing)")
	solverCmd := flag.String("solver", os.Getenv("DUTYPLAN_SOLVER"), "command that starts the planner, e.g. \"python -m dutyplan_solver\" (default $DUTYPLAN_SOLVER; empty = no planning)")
	workers := flag.Int("workers", min(runtime.NumCPU(), 8), "threads the planner may use")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	st, err := store.Open(*dbPath)
	if err != nil {
		log.Error("open database", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// A run that was going when the server stopped can never finish: say so instead of leaving it «running».
	if n, err := st.FailRunningRuns(context.Background()); err != nil {
		log.Error("close interrupted runs", "err", err)
		os.Exit(1)
	} else if n > 0 {
		log.Warn("marked interrupted runs as failed", "runs", n)
	}
	var solver planner.Solver
	if argv := strings.Fields(*solverCmd); len(argv) > 0 {
		solver = planner.ExecSolver{Argv: argv}
	} else {
		log.Warn("no planner configured (--solver / DUTYPLAN_SOLVER): everything but «Plan» works")
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
	handler := api.New(st, planner.New(st, solver, *workers, log), web.Files, log)
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	fmt.Printf("dutyplan: http://%s\n", ln.Addr()) // the one line scripts and people look for

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		handler.CloseStreams() // the open streams would otherwise hold the shutdown up
		srv.Shutdown(ctx)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
