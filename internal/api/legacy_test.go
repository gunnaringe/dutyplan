package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

// The tests were written against the JSON API that came before Connect (2026-11-23). To show that the move changed no
// behaviour, they still speak it: legacy() turns each of the old requests into the Connect call that replaced it, and the
// answer back into the old shape and status. The calls themselves go through the real handler (codec, interceptor,
// access, errors), so the tests check the Connect API; only the addressing is translated.

// route: the old method and path, with each {name} a field of the new request (ids are numbers, org/plan text); unwrap
// is the field of the answer that held the whole old answer; status the old status of success.
type route struct {
	method, path, proc, unwrap string
	status                     int
}

var legacyRoutes = []route{
	{"GET", "/api/health", "AccountService/Health", "", 200},
	{"GET", "/api/me", "AccountService/Me", "", 200},
	{"GET", "/api/orgs", "AccountService/ListOrgs", "orgs", 200},
	{"POST", "/api/orgs", "AccountService/CreateOrg", "org", 201},

	{"GET", "/api/orgs/{org}", "OrgService/GetOrg", "", 200},
	{"PATCH", "/api/orgs/{org}", "OrgService/RenameOrg", "org", 200},
	{"DELETE", "/api/orgs/{org}", "OrgService/DeleteOrg", "", 204},
	{"GET", "/api/orgs/{org}/members", "OrgService/ListMembers", "members", 200},
	{"PATCH", "/api/orgs/{org}/members/{id}", "OrgService/SetMemberRole", "", 204},
	{"DELETE", "/api/orgs/{org}/members/{id}", "OrgService/RemoveMember", "", 204},
	{"GET", "/api/orgs/{org}/links", "OrgService/ListLinks", "links", 200},
	{"POST", "/api/orgs/{org}/links", "OrgService/CreateLink", "link", 201},
	{"DELETE", "/api/orgs/{org}/links/{id}", "OrgService/DeleteLink", "", 204},

	{"GET", "/api/orgs/{org}/plans/{plan}/tasks", "SetupService/ListTasks", "tasks", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/tasks", "SetupService/CreateTask", "task", 201},
	{"PATCH", "/api/orgs/{org}/plans/{plan}/tasks/{id}", "SetupService/UpdateTask", "task", 200},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/tasks/{id}", "SetupService/DeleteTask", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/tasks/{id}/position", "SetupService/MoveTask", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/combinations", "SetupService/ListCombinations", "combinations", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/combinations", "SetupService/SetCombination", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/tasks/{task}/combinations", "SetupService/SetTaskCombinations", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/people", "SetupService/ListPeople", "people", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/people", "SetupService/CreatePerson", "person", 201},
	{"PATCH", "/api/orgs/{org}/plans/{plan}/people/{id}", "SetupService/UpdatePerson", "person", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/people/{id}/position", "SetupService/MovePerson", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/people/{person}/tasks/{task}", "PlanService/SetPersonTask", "", 204},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/people/{person}/tasks/{task}", "PlanService/RemovePersonTask", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/sections", "SetupService/ListSections", "sections", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/sections", "SetupService/CreateSection", "section", 201},
	{"PATCH", "/api/orgs/{org}/plans/{plan}/sections/{id}", "SetupService/RenameSection", "section", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/sections/{id}/position", "SetupService/MoveSection", "", 204},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/sections/{id}", "SetupService/DeleteSection", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/task-sections", "SetupService/ListTaskSections", "sections", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/task-sections", "SetupService/CreateTaskSection", "section", 201},
	{"PATCH", "/api/orgs/{org}/plans/{plan}/task-sections/{id}", "SetupService/RenameTaskSection", "section", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/task-sections/{id}/position", "SetupService/MoveTaskSection", "", 204},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/task-sections/{id}", "SetupService/DeleteTaskSection", "", 204},

	{"GET", "/api/orgs/{org}/plans", "PlanService/ListPlans", "plans", 200},
	{"POST", "/api/orgs/{org}/plans", "PlanService/CreatePlan", "plan", 201},
	{"POST", "/api/orgs/{org}/plans/{plan}/duplicate", "PlanService/DuplicatePlan", "plan", 201},
	{"GET", "/api/orgs/{org}/plans/{plan}", "PlanService/GetPlan", "plan", 200},
	{"PATCH", "/api/orgs/{org}/plans/{plan}", "PlanService/RenamePlan", "plan", 200},
	{"DELETE", "/api/orgs/{org}/plans/{plan}", "PlanService/DeletePlan", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/events", "PlanService/ListEvents", "events", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/events", "PlanService/AddEvents", "events", 201},
	{"POST", "/api/orgs/{org}/plans/{plan}/events/{id}/copy", "PlanService/CopyEvent", "events", 201},
	{"PATCH", "/api/orgs/{org}/plans/{plan}/events/{id}", "PlanService/UpdateEvent", "event", 200},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/events/{id}", "PlanService/DeleteEvent", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/places", "PlanService/ListPlaces", "places", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/events/{event}/places/{task}", "PlanService/SetPlaces", "", 204},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/events/{event}/places/{task}", "PlanService/ResetPlaces", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/places", "PlanService/SetPlacesMany", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/fields", "PlanService/ListFields", "", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/fields", "PlanService/AddField", "field", 201},
	{"PATCH", "/api/orgs/{org}/plans/{plan}/fields/{id}", "PlanService/RenameField", "field", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/fields/{id}/position", "PlanService/MoveField", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/fields/{id}/frozen", "PlanService/FreezeField", "", 204},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/fields/{id}", "PlanService/DeleteField", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/events/{event}/fields/{field}", "PlanService/SetFieldValue", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/answers", "PlanService/ListAnswers", "", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/people/{person}/availability", "PlanService/SetAvailability", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/events/{event}/notes/{person}", "PlanService/SetNote", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/answers", "PlanService/SetAnswers", "", 204},

	{"GET", "/api/orgs/{org}/plans/{plan}/cells", "DraftService/ListCells", "cells", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/cells", "DraftService/AddCell", "cell", 201},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/cells/{id}", "DraftService/DeleteCell", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/pools", "DraftService/ListPools", "pools", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/pools", "DraftService/SetPool", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/locks", "DraftService/SetLocks", "", 204},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/draft", "DraftService/SetDraftAside", "", 204},
	{"PUT", "/api/orgs/{org}/plans/{plan}/draft", "DraftService/ClearDraftAside", "", 204},
	{"GET", "/api/orgs/{org}/plans/{plan}/weights", "DraftService/GetWeights", "", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/weights", "DraftService/SetWeights", "", 200},
	{"POST", "/api/orgs/{org}/plans/{plan}/runs", "DraftService/StartRun", "", 202},
	{"GET", "/api/orgs/{org}/plans/{plan}/runs", "DraftService/ListRuns", "runs", 200},
	{"GET", "/api/orgs/{org}/plans/{plan}/runs/{id}", "DraftService/GetRun", "", 200},
	{"PUT", "/api/orgs/{org}/plans/{plan}/runs/{id}/pin", "DraftService/PinRun", "", 204},
	{"DELETE", "/api/orgs/{org}/plans/{plan}/runs/{id}/pin", "DraftService/UnpinRun", "", 204},
}

// match finds the route of an old request, with the values of its {names}.
func match(method, path string) (route, map[string]string, bool) {
	segs := strings.Split(strings.TrimSuffix(path, "/"), "/")
	for _, r := range legacyRoutes {
		if r.method != method {
			continue
		}
		ps := strings.Split(r.path, "/")
		if len(ps) != len(segs) {
			continue
		}
		vals, ok := map[string]string{}, true
		for i, p := range ps {
			if strings.HasPrefix(p, "{") {
				vals[p[1:len(p)-1]] = segs[i]
			} else if p != segs[i] {
				ok = false
				break
			}
		}
		if ok {
			return r, vals, true
		}
	}
	return route{}, nil, false
}

// legacy wraps the app's handler so the tests' old requests reach their Connect calls.
func legacy(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/stream") {
			h.ServeHTTP(w, r)
			return
		}
		rt, vals, ok := match(r.Method, r.URL.EscapedPath())
		if !ok {
			h.ServeHTTP(w, r)
			return
		}
		body := map[string]any{}
		raw, _ := io.ReadAll(r.Body)
		if len(bytes.TrimSpace(raw)) > 0 {
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.UseNumber()
			if err := dec.Decode(&body); err != nil { // not a JSON object: send it as it is, the API refuses it
				body = nil
			}
		}
		for k, v := range vals {
			switch k {
			case "org", "plan":
				body[k] = v
			default:
				if v == "latest" {
					v = "0"
				}
				n, err := strconv.ParseInt(v, 10, 32)
				if err != nil {
					writeJSON(w, 404, apiError{Error: "not_found", Message: "not found"})
					return
				}
				body[k] = n
			}
		}
		if c := r.URL.Query().Get("confirm"); c != "" || rt.proc == "OrgService/DeleteOrg" {
			body["confirm"] = c
		}
		if l := r.URL.Query().Get("lang"); l != "" {
			body["lang"] = l
		}
		out := raw // as sent when nothing is added (a padded body stays padded)
		if body != nil && (len(vals) > 0 || len(r.URL.RawQuery) > 0) {
			out, _ = json.Marshal(body)
		}
		req := r.Clone(r.Context())
		req.Method = "POST"
		req.URL.Path, req.URL.RawPath, req.URL.RawQuery = svc+rt.proc, "", ""
		req.RequestURI = ""
		req.Body = io.NopCloser(bytes.NewReader(out))
		req.ContentLength = int64(len(out))
		if r.Method == "GET" || r.Header.Get("Content-Type") == "" && len(raw) == 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		if rec.Code != 200 {
			var e struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Details []struct {
					Debug struct {
						Error  string         `json:"error"`
						Code   string         `json:"code"`
						Params map[string]any `json:"params"`
					} `json:"debug"`
				} `json:"details"`
			}
			if json.Unmarshal(rec.Body.Bytes(), &e) != nil || e.Code == "" { // not Connect's (415, 404 of the mux)
				w.WriteHeader(rec.Code)
				w.Write(rec.Body.Bytes())
				return
			}
			old := apiError{Error: e.Code, Message: e.Message}
			if e.Code == "invalid_argument" {
				old.Error = "invalid"
			}
			if len(e.Details) > 0 {
				d := e.Details[0].Debug
				old.Error, old.Code, old.Params = d.Error, d.Code, d.Params
			}
			writeJSON(w, rec.Code, old)
			return
		}
		if rt.status == 204 {
			w.Header().Del("Content-Type")
			w.WriteHeader(204)
			return
		}
		b := rec.Body.Bytes()
		if rt.unwrap != "" {
			var m map[string]json.RawMessage
			json.Unmarshal(b, &m)
			b = m[rt.unwrap]
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(rt.status)
		w.Write(b)
	})
}
