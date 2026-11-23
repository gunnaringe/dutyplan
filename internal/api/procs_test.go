package api

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	v1 "github.com/gunnaringe/dutyplan/gen/dutyplan/v1"
)

// Access is decided from procs alone (procs.go), so every procedure of the API needs a line there, and the request of
// every procedure about an organisation or a plan must carry `org` (and `plan`) for the gate to read. A procedure the
// table does not know is nobody's, and a line for a procedure that is gone is a mistake.
func TestEveryProcedureHasALine(t *testing.T) {
	services := v1.File_dutyplan_v1_dutyplan_proto.Services()
	seen := map[string]bool{}
	for i := range services.Len() {
		sv := services.Get(i)
		for j := range sv.Methods().Len() {
			m := sv.Methods().Get(j)
			name := "/" + string(sv.FullName()) + "/" + string(m.Name())
			seen[name] = true
			p, ok := procs[name]
			if !ok {
				t.Errorf("%s has no line in procs", name)
				continue
			}
			fields := m.Input().Fields()
			isString := func(f string) bool {
				fd := fields.ByName(protoreflect.Name(f))
				return fd != nil && fd.Kind() == protoreflect.StringKind && fd.Number() <= 2
			}
			switch p.scope {
			case scopeOrg:
				if !isString("org") {
					t.Errorf("%s is about an organisation but its request has no `org` (field 1)", name)
				}
			case scopePlan:
				if !isString("org") || !isString("plan") {
					t.Errorf("%s is about a plan but its request lacks `org` and `plan` (fields 1 and 2)", name)
				}
			case scopeNone:
				if fields.ByName("org") != nil {
					t.Errorf("%s names an organisation but its line says it has none", name)
				}
			}
			if p.open && (p.write || p.scope != scopeNone) {
				t.Errorf("%s: only reading without an organisation can be open", name)
			}
			if p.answer && (p.scope != scopePlan || !p.write) {
				t.Errorf("%s: an answer link writes only in its plan", name)
			}
			if p.self && !p.admin {
				t.Errorf("%s: self only loosens an admin-only line", name)
			}
		}
	}
	for name := range procs {
		if !seen[name] {
			t.Errorf("procs has %s, which the API does not have", name)
		}
		if !strings.HasPrefix(name, svc) {
			t.Errorf("%s is not of dutyplan.v1", name)
		}
	}
}
