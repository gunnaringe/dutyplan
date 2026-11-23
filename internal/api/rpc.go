package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	dutyplanv1 "github.com/gunnaringe/dutyplan/gen/dutyplan/v1"
	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

// The API is Connect (proto/dutyplan/v1/dutyplan.proto) with JSON only: POST /dutyplan.v1.<Service>/<Method>.
//
// The JSON on the wire is the one the page and the agent guide use: the field names of the proto (snake_case), every
// field sent even when empty (so the page never meets a missing list or false), and unknown fields refused so a typo
// does not pass silently. Ids are int32 in the proto, so they are numbers in JSON (int64 would be strings); SQLite's
// ids never come near 2³¹, and a bigger one would fail loudly in out().

// jsonCodec is protojson with those options. Connect knows JSON by two names (with and without the charset).
type jsonCodec struct{ name string }

var (
	marshalJSON   = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}
	unmarshalJSON = protojson.UnmarshalOptions{DiscardUnknown: false}
)

func (c jsonCodec) Name() string { return c.name }

func (c jsonCodec) Marshal(v any) ([]byte, error) {
	m, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("not a proto message: %T", v)
	}
	b, err := marshalJSON.Marshal(m)
	if err != nil {
		return nil, err
	}
	// protojson puts random spaces in on purpose (so nobody depends on its bytes); the answers are compact and stable
	// instead, like the API's before Connect, so scripts that read them with sed and grep keep working.
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (c jsonCodec) Unmarshal(b []byte, v any) error {
	m, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("not a proto message: %T", v)
	}
	if len(b) == 0 { // an empty body is an empty message
		return nil
	}
	return unmarshalJSON.Unmarshal(b, m)
}

func (c jsonCodec) IsBinary() bool { return false }

// requireJSON caps the body at what the procedure needs (procs.go: the bulk writes of a selection are bigger), lets
// the gate decide access, and then takes JSON only (Connect would also take binary protobuf, which the page and the
// guide never send). The order is the old API's: who may call first, then what they sent.
func requireJSON(gate func(http.Handler) http.Handler, next http.Handler) http.Handler {
	json := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); len(ct) < 16 || ct[:16] != "application/json" {
			writeJSON(w, 415, apiError{Error: "unsupported_media_type", Message: "send Content-Type: application/json"})
			return
		}
		next.ServeHTTP(w, r)
	})
	gated := gate(json)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		max := int64(64 << 10)
		if p, ok := procs[r.URL.Path]; ok && p.big {
			max = 512 << 10
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, max))
		if err != nil {
			writeJSON(w, 400, apiError{Error: "invalid", Message: "the body is too big"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		gated.ServeHTTP(w, r)
	})
}

// out fills a response message from a value of the store or the planner, through their JSON (the proto mirrors it field
// by field). A field the message does not have is an error, so the two cannot drift apart unnoticed (rpc_test.go).
func out[M proto.Message](v any, m M) (M, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return m, err
	}
	if err := unmarshalJSON.Unmarshal(b, m); err != nil {
		return m, fmt.Errorf("the answer does not fit %T: %w", m, err)
	}
	return m, nil
}

// in turns a request's part into a value the store takes, through JSON (for lists and maps of the store's own types).
func in(m proto.Message, v any) error {
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// structOf is a map as a google.protobuf.Struct (the weights, the parameters of an error).
func structOf(v map[string]any) (*structpb.Struct, error) {
	if v == nil {
		v = map[string]any{}
	}
	b, err := json.Marshal(v) // numbers of any Go type become JSON numbers first
	if err != nil {
		return nil, err
	}
	s := &structpb.Struct{}
	return s, protojson.Unmarshal(b, s)
}

// ---- errors ---------------------------------------------------------------------------------------------------------

// apiError is the body of a refusal outside Connect (the stream, a wrong content type, a panic, an unknown /api path).
type apiError struct {
	Error   string         `json:"error"`
	Code    string         `json:"code,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Message string         `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// problem builds a Connect error with the Problem detail the page reads (kind, code, parameters).
func problem(code connect.Code, kind, appCode string, params map[string]any, msg string) *connect.Error {
	e := connect.NewError(code, errors.New(msg))
	p := &dutyplanv1.Problem{Error: kind, Code: appCode}
	if params != nil {
		if s, err := structOf(params); err == nil {
			p.Params = s
		}
	}
	if d, err := connect.NewErrorDetail(p); err == nil {
		e.AddDetail(d)
	}
	return e
}

// fail turns an error of the store or the planner into the Connect error the page understands: 404 not_found, 409
// conflict or busy, 400 invalid, 503 unavailable, else 500 (logged).
func fail(log *slog.Logger, err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce
	}
	code, params := "", map[string]any(nil)
	var c *store.Coded
	if errors.As(err, &c) {
		code, params = c.Code, c.Params
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return problem(connect.CodeNotFound, "not_found", "", nil, "not found")
	case errors.Is(err, store.ErrConflict):
		return problem(connect.CodeAborted, "conflict", code, params, err.Error())
	case errors.Is(err, store.ErrInvalid):
		return problem(connect.CodeInvalidArgument, "invalid", code, params, err.Error())
	case errors.Is(err, planner.ErrUnavailable):
		return problem(connect.CodeUnavailable, "unavailable", "", nil, err.Error())
	case errors.Is(err, planner.ErrBusy):
		return problem(connect.CodeAborted, "busy", "", nil, err.Error())
	default:
		log.Error("request failed", "err", err)
		return problem(connect.CodeInternal, "internal", "", nil, "internal error")
	}
}

// writeConnectErr writes a Connect error outside Connect (the stream): its status and the same body the page reads.
func writeConnectErr(w http.ResponseWriter, err error) {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		ce = connect.NewError(connect.CodeInternal, err)
	}
	e := apiError{Error: "internal", Message: ce.Message()}
	for _, d := range ce.Details() {
		if v, err := d.Value(); err == nil {
			if p, ok := v.(*dutyplanv1.Problem); ok {
				e.Error, e.Code, e.Params = p.Error, p.Code, p.Params.AsMap()
			}
		}
	}
	status := map[connect.Code]int{connect.CodeNotFound: 404, connect.CodeUnauthenticated: 401, connect.CodePermissionDenied: 403,
		connect.CodeInvalidArgument: 400, connect.CodeUnavailable: 503, connect.CodeAborted: 409, connect.CodeResourceExhausted: 429}[ce.Code()]
	if status == 0 {
		status = 500
	}
	writeJSON(w, status, e)
}

// invalid is a refused request the store would not see (a required field left out).
func invalid(msg string) error {
	return problem(connect.CodeInvalidArgument, "invalid", "", nil, msg)
}

// procName is the last part of a procedure («/dutyplan.v1.PlanService/SetPlaces» → «SetPlaces»).
func procName(p string) string { return p[strings.LastIndex(p, "/")+1:] }
