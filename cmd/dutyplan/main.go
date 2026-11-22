// Command dutyplan serves the duty-planning app: one binary, SQLite, embedded frontend, signing in with OpenID Connect.
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
	// dutyplan health [url]: asks a running server whether it is well (exit 0) or not (1), for a container's health check,
	// which then needs no shell, curl or wget in the image.
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(health(os.Args[2:]))
	}
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on (port 0 picks a free one)")
	dbPath := flag.String("db", "dutyplan.db", "SQLite database file (created if missing)")
	solverCmd := flag.String("solver", os.Getenv("DUTYPLAN_SOLVER"), "command that starts the planner, e.g. \"python -m dutyplan_solver\" (default $DUTYPLAN_SOLVER; empty = no planning)")
	workers := flag.Int("workers", min(runtime.NumCPU(), 8), "threads the planner may use")
	issuer := flag.String("oidc-issuer", os.Getenv("DUTYPLAN_OIDC_ISSUER"), "OpenID Connect provider to sign in with (default $DUTYPLAN_OIDC_ISSUER)")
	clientID := flag.String("oidc-client-id", os.Getenv("DUTYPLAN_OIDC_CLIENT_ID"), "the app's client id at the provider (default $DUTYPLAN_OIDC_CLIENT_ID)")
	secretFile := flag.String("oidc-client-secret-file", os.Getenv("DUTYPLAN_OIDC_CLIENT_SECRET_FILE"), "file with the client secret (default $DUTYPLAN_OIDC_CLIENT_SECRET_FILE)")
	publicURL := flag.String("public-url", os.Getenv("DUTYPLAN_PUBLIC_URL"), "the app's address as people open it, e.g. https://dutyplan.example.org/ (default $DUTYPLAN_PUBLIC_URL)")
	signupURL := flag.String("oidc-signup-url", os.Getenv("DUTYPLAN_OIDC_SIGNUP_URL"), "optional: where to make an account at the provider, offered on the start page (default $DUTYPLAN_OIDC_SIGNUP_URL)")
	noLogin := flag.Bool("no-login", os.Getenv("DUTYPLAN_NO_LOGIN") == "1", "no signing in: everyone who reaches the address may do everything (default $DUTYPLAN_NO_LOGIN=1)")
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
	pl := planner.New(st, solver, *workers, log)
	var handler *api.Handler
	switch {
	case *issuer != "" && !*noLogin:
		var secret []byte
		if *secretFile != "" {
			if secret, err = os.ReadFile(*secretFile); err != nil {
				log.Error("read the client secret", "err", err)
				os.Exit(1)
			}
		}
		handler, err = api.NewWithLogin(st, pl, web.Files, log, api.Login{Issuer: *issuer, ClientID: *clientID,
			ClientSecret: strings.TrimSpace(string(secret)), PublicURL: *publicURL, SignupURL: *signupURL})
		if err != nil {
			log.Error("login", "err", err)
			os.Exit(1)
		}
	case *noLogin || loopback(ln.Addr()):
		if !*noLogin {
			log.Warn("no login configured (--oidc-issuer): fine on this machine only, everyone who reaches it may do everything")
		}
		handler = api.New(st, pl, web.Files, log)
	default:
		log.Error("no login configured: give --oidc-issuer (and the client and public URL), or --no-login if something in front does the signing in")
		os.Exit(1)
	}
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

// loopback: only this machine can reach the address.
func loopback(a net.Addr) bool {
	t, ok := a.(*net.TCPAddr)
	return ok && t.IP.IsLoopback()
}

func health(args []string) int {
	url := "http://127.0.0.1:8080/api/health"
	if len(args) > 0 {
		url = args[0]
	}
	c := http.Client{Timeout: 5 * time.Second}
	res, err := c.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "health:", err)
		return 1
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "health:", res.Status)
		return 1
	}
	return 0
}
