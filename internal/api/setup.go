package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/gunnaringe/dutyplan/gen/dutyplan/v1"
	"github.com/gunnaringe/dutyplan/internal/store"
)

// setupSvc: the organisation's own tasks, combinations, people and their tasks, and the sections of people and of tasks.
type setupSvc struct{ *server }

// required: a field the request must send (proto3 cannot say so); the error the store would give for a bad value.
func required[T any](v *T, code, msg string, params map[string]any) (T, error) {
	if v == nil {
		var zero T
		return zero, store.Invalid(code, params, "%s", msg)
	}
	return *v, nil
}

func positionOf(v *int32) (int, error) {
	p, err := required(v, "position_invalid", "send the new position in position", nil)
	return int(p), err
}

// ---- tasks ----------------------------------------------------------------------------------------------------------

func (s setupSvc) ListTasks(ctx context.Context, req *connect.Request[v1.ListTasksRequest]) (*connect.Response[v1.ListTasksResponse], error) {
	v, err := s.st.ListTasks(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListTasksResponse](s.server, map[string]any{"tasks": v}, err)
}

func (s setupSvc) CreateTask(ctx context.Context, req *connect.Request[v1.CreateTaskRequest]) (*connect.Response[v1.CreateTaskResponse], error) {
	t, err := s.st.CreateTaskIn(ctx, req.Msg.Org, req.Msg.Plan, req.Msg.Name, int64(req.Msg.Section))
	return conv[v1.CreateTaskResponse](s.server, map[string]any{"task": t}, err)
}

func (s setupSvc) UpdateTask(ctx context.Context, req *connect.Request[v1.UpdateTaskRequest]) (*connect.Response[v1.UpdateTaskResponse], error) {
	t, err := s.st.UpdateTask(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), store.TaskPatch{Name: req.Msg.Name, Planned: req.Msg.Planned})
	return conv[v1.UpdateTaskResponse](s.server, map[string]any{"task": t}, err)
}

func (s setupSvc) DeleteTask(ctx context.Context, req *connect.Request[v1.DeleteTaskRequest]) (*connect.Response[v1.DeleteTaskResponse], error) {
	return done[v1.DeleteTaskResponse](s.server, s.st.DeleteTask(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id)))
}

// MoveTask puts a task at a position in a section (0 = none; left out: the one it is in).
func (s setupSvc) MoveTask(ctx context.Context, req *connect.Request[v1.MoveTaskRequest]) (*connect.Response[v1.MoveTaskResponse], error) {
	pos, err := positionOf(req.Msg.Position)
	if err != nil {
		return nil, fail(s.log, err)
	}
	var section *int64
	if req.Msg.Section != nil {
		x := int64(*req.Msg.Section)
		section = &x
	}
	return done[v1.MoveTaskResponse](s.server, s.st.MoveTask(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), section, pos))
}

func (s setupSvc) ListCombinations(ctx context.Context, req *connect.Request[v1.ListCombinationsRequest]) (*connect.Response[v1.ListCombinationsResponse], error) {
	v, err := s.st.ListCombinations(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListCombinationsResponse](s.server, map[string]any{"combinations": v}, err)
}

func (s setupSvc) SetCombination(ctx context.Context, req *connect.Request[v1.SetCombinationRequest]) (*connect.Response[v1.SetCombinationResponse], error) {
	m := req.Msg
	return done[v1.SetCombinationResponse](s.server, s.st.SetCombination(ctx, m.Org, m.Plan, int64(m.A), int64(m.B), m.Level))
}

// SetTaskCombinations sets one task against every other.
func (s setupSvc) SetTaskCombinations(ctx context.Context, req *connect.Request[v1.SetTaskCombinationsRequest]) (*connect.Response[v1.SetTaskCombinationsResponse], error) {
	m := req.Msg
	return done[v1.SetTaskCombinationsResponse](s.server, s.st.SetTaskCombinations(ctx, m.Org, m.Plan, int64(m.Task), m.Level))
}

// ---- people ---------------------------------------------------------------------------------------------------------

func (s setupSvc) ListPeople(ctx context.Context, req *connect.Request[v1.ListPeopleRequest]) (*connect.Response[v1.ListPeopleResponse], error) {
	v, err := s.st.ListPeople(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListPeopleResponse](s.server, map[string]any{"people": v}, err)
}

func (s setupSvc) CreatePerson(ctx context.Context, req *connect.Request[v1.CreatePersonRequest]) (*connect.Response[v1.CreatePersonResponse], error) {
	p, err := s.st.CreatePersonIn(ctx, req.Msg.Org, req.Msg.Plan, req.Msg.Name, int64(req.Msg.Section))
	return conv[v1.CreatePersonResponse](s.server, map[string]any{"person": p}, err)
}

func (s setupSvc) UpdatePerson(ctx context.Context, req *connect.Request[v1.UpdatePersonRequest]) (*connect.Response[v1.UpdatePersonResponse], error) {
	p, err := s.st.UpdatePerson(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), store.PersonPatch{Name: req.Msg.Name, Proposable: req.Msg.Proposable})
	return conv[v1.UpdatePersonResponse](s.server, map[string]any{"person": p}, err)
}

// MovePerson puts a person in a section (0 = none) at a position among its people.
func (s setupSvc) MovePerson(ctx context.Context, req *connect.Request[v1.MovePersonRequest]) (*connect.Response[v1.MovePersonResponse], error) {
	pos, err := positionOf(req.Msg.Position)
	if err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.MovePersonResponse](s.server, s.st.MovePerson(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), int64(req.Msg.Section), pos))
}

// ---- sections: the same code for people's (store.People) and tasks' (store.Tasks) ---------------------------------------

func (s setupSvc) listSections(ctx context.Context, of store.SectionsOf, org, plan string) ([]store.Section, error) {
	return s.st.ListSectionsOf(ctx, of, org, plan)
}

func (s setupSvc) ListSections(ctx context.Context, req *connect.Request[v1.ListSectionsRequest]) (*connect.Response[v1.ListSectionsResponse], error) {
	v, err := s.listSections(ctx, store.People, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListSectionsResponse](s.server, map[string]any{"sections": v}, err)
}

func (s setupSvc) CreateSection(ctx context.Context, req *connect.Request[v1.CreateSectionRequest]) (*connect.Response[v1.CreateSectionResponse], error) {
	v, err := s.st.CreateSectionOf(ctx, store.People, req.Msg.Org, req.Msg.Plan, req.Msg.Name)
	return conv[v1.CreateSectionResponse](s.server, map[string]any{"section": v}, err)
}

func (s setupSvc) RenameSection(ctx context.Context, req *connect.Request[v1.RenameSectionRequest]) (*connect.Response[v1.RenameSectionResponse], error) {
	v, err := s.st.RenameSectionOf(ctx, store.People, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), req.Msg.Name)
	return conv[v1.RenameSectionResponse](s.server, map[string]any{"section": v}, err)
}

func (s setupSvc) MoveSection(ctx context.Context, req *connect.Request[v1.MoveSectionRequest]) (*connect.Response[v1.MoveSectionResponse], error) {
	pos, err := positionOf(req.Msg.Position)
	if err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.MoveSectionResponse](s.server, s.st.MoveSectionOf(ctx, store.People, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), pos))
}

func (s setupSvc) DeleteSection(ctx context.Context, req *connect.Request[v1.DeleteSectionRequest]) (*connect.Response[v1.DeleteSectionResponse], error) {
	return done[v1.DeleteSectionResponse](s.server, s.st.DeleteSectionOf(ctx, store.People, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id)))
}

func (s setupSvc) ListTaskSections(ctx context.Context, req *connect.Request[v1.ListTaskSectionsRequest]) (*connect.Response[v1.ListTaskSectionsResponse], error) {
	v, err := s.listSections(ctx, store.Tasks, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListTaskSectionsResponse](s.server, map[string]any{"sections": v}, err)
}

func (s setupSvc) CreateTaskSection(ctx context.Context, req *connect.Request[v1.CreateTaskSectionRequest]) (*connect.Response[v1.CreateTaskSectionResponse], error) {
	v, err := s.st.CreateSectionOf(ctx, store.Tasks, req.Msg.Org, req.Msg.Plan, req.Msg.Name)
	return conv[v1.CreateTaskSectionResponse](s.server, map[string]any{"section": v}, err)
}

func (s setupSvc) RenameTaskSection(ctx context.Context, req *connect.Request[v1.RenameTaskSectionRequest]) (*connect.Response[v1.RenameTaskSectionResponse], error) {
	v, err := s.st.RenameSectionOf(ctx, store.Tasks, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), req.Msg.Name)
	return conv[v1.RenameTaskSectionResponse](s.server, map[string]any{"section": v}, err)
}

func (s setupSvc) MoveTaskSection(ctx context.Context, req *connect.Request[v1.MoveTaskSectionRequest]) (*connect.Response[v1.MoveTaskSectionResponse], error) {
	pos, err := positionOf(req.Msg.Position)
	if err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.MoveTaskSectionResponse](s.server, s.st.MoveSectionOf(ctx, store.Tasks, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), pos))
}

func (s setupSvc) DeleteTaskSection(ctx context.Context, req *connect.Request[v1.DeleteTaskSectionRequest]) (*connect.Response[v1.DeleteTaskSectionResponse], error) {
	return done[v1.DeleteTaskSectionResponse](s.server, s.st.DeleteSectionOf(ctx, store.Tasks, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id)))
}
