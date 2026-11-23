package api

import (
	"context"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
)

// Reflection answers through the whole handler (the wrappers must let a stream through), lists the five services, and
// gives the schema with its comments (which the generated code's descriptors do not have).
func TestReflectionListsTheServicesWithComments(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewUnstartedServer(e.app)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	stream := grpcreflect.NewClient(srv.Client(), srv.URL, connect.WithGRPC()).NewStream(context.Background())
	defer stream.Close()
	names, err := stream.ListServices()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range names {
		got = append(got, string(n))
	}
	slices.Sort(got)
	want := []string{"dutyplan.v1.AccountService", "dutyplan.v1.DraftService", "dutyplan.v1.OrgService",
		"dutyplan.v1.PlanService", "dutyplan.v1.SetupService"}
	if !slices.Equal(got, want) {
		t.Fatalf("services: got %v, want %v", got, want)
	}
	files, err := stream.FileContainingSymbol("dutyplan.v1.DraftService.StartRun")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range files {
		for _, l := range f.GetSourceCodeInfo().GetLocation() {
			if strings.Contains(l.GetLeadingComments(), "poll GetRun") {
				found = true
			}
		}
	}
	if !found {
		t.Error("the schema reflection gives has no comments (StartRun's «poll GetRun»)")
	}
}
