package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/gunnaringe/dutyplan/gen/dutyplan/v1"
	"github.com/gunnaringe/dutyplan/internal/store"
)

// planSvc: plans, their events, and the table: places, details rows, answers and notes.
type planSvc struct{ *server }

// ---- plans ----------------------------------------------------------------------------------------------------------

func (s planSvc) ListPlans(ctx context.Context, req *connect.Request[v1.ListPlansRequest]) (*connect.Response[v1.ListPlansResponse], error) {
	v, err := s.st.ListPlans(ctx, req.Msg.Org)
	return conv[v1.ListPlansResponse](s.server, map[string]any{"plans": v}, err)
}

func (s planSvc) CreatePlan(ctx context.Context, req *connect.Request[v1.CreatePlanRequest]) (*connect.Response[v1.CreatePlanResponse], error) {
	p, err := s.st.CreatePlan(ctx, req.Msg.Org, req.Msg.Name)
	return conv[v1.CreatePlanResponse](s.server, map[string]any{"plan": p}, err)
}

func (s planSvc) GetPlan(ctx context.Context, req *connect.Request[v1.GetPlanRequest]) (*connect.Response[v1.GetPlanResponse], error) {
	p, err := s.st.GetPlan(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.GetPlanResponse](s.server, map[string]any{"plan": p}, err)
}

// RenamePlan changes the name only; the address stays.
func (s planSvc) RenamePlan(ctx context.Context, req *connect.Request[v1.RenamePlanRequest]) (*connect.Response[v1.RenamePlanResponse], error) {
	p, err := s.st.RenamePlan(ctx, req.Msg.Org, req.Msg.Plan, req.Msg.Name)
	return conv[v1.RenamePlanResponse](s.server, map[string]any{"plan": p}, err)
}

func (s planSvc) DeletePlan(ctx context.Context, req *connect.Request[v1.DeletePlanRequest]) (*connect.Response[v1.DeletePlanResponse], error) {
	return done[v1.DeletePlanResponse](s.server, s.st.DeletePlan(ctx, req.Msg.Org, req.Msg.Plan))
}

// ---- events ---------------------------------------------------------------------------------------------------------

func (s planSvc) ListEvents(ctx context.Context, req *connect.Request[v1.ListEventsRequest]) (*connect.Response[v1.ListEventsResponse], error) {
	v, err := s.st.ListEvents(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListEventsResponse](s.server, map[string]any{"events": v}, err)
}

// AddEvents: one event, or every N weeks until a date.
func (s planSvc) AddEvents(ctx context.Context, req *connect.Request[v1.AddEventsRequest]) (*connect.Response[v1.AddEventsResponse], error) {
	m := req.Msg
	evs, err := s.st.AddEvents(ctx, m.Org, m.Plan, store.NewEvents{Date: m.Date, Until: m.Until, Every: int(m.Every), StartTime: m.StartTime, EndTime: m.EndTime})
	return conv[v1.AddEventsResponse](s.server, map[string]any{"events": evs}, err)
}

// CopyEvent: copies of an event on dates, or every N weeks until a date.
func (s planSvc) CopyEvent(ctx context.Context, req *connect.Request[v1.CopyEventRequest]) (*connect.Response[v1.CopyEventResponse], error) {
	m := req.Msg
	evs, err := s.st.CopyEvent(ctx, m.Org, m.Plan, int64(m.Id), store.CopyOf{Dates: m.Dates, Until: m.Until, Every: int(m.Every), Times: m.Times,
		StartTime: m.StartTime, EndTime: m.EndTime, Details: m.Details, Places: m.Places, Hand: m.Hand})
	return conv[v1.CopyEventResponse](s.server, map[string]any{"events": evs}, err)
}

func (s planSvc) UpdateEvent(ctx context.Context, req *connect.Request[v1.UpdateEventRequest]) (*connect.Response[v1.UpdateEventResponse], error) {
	m := req.Msg
	ev, err := s.st.UpdateEvent(ctx, m.Org, m.Plan, int64(m.Id), store.EventPatch{StartTime: m.StartTime, EndTime: m.EndTime})
	return conv[v1.UpdateEventResponse](s.server, map[string]any{"event": ev}, err)
}

func (s planSvc) DeleteEvent(ctx context.Context, req *connect.Request[v1.DeleteEventRequest]) (*connect.Response[v1.DeleteEventResponse], error) {
	return done[v1.DeleteEventResponse](s.server, s.st.DeleteEvent(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id)))
}

// ---- places ---------------------------------------------------------------------------------------------------------

func (s planSvc) ListPlaces(ctx context.Context, req *connect.Request[v1.ListPlacesRequest]) (*connect.Response[v1.ListPlacesResponse], error) {
	v, err := s.st.ListEventPlaces(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListPlacesResponse](s.server, map[string]any{"places": v}, err)
}

// SetPlaces: the number must be sent (a missing one is not 0, which would reset).
func (s planSvc) SetPlaces(ctx context.Context, req *connect.Request[v1.SetPlacesRequest]) (*connect.Response[v1.SetPlacesResponse], error) {
	m := req.Msg
	n, err := required(m.Slots, "places_invalid", "send the number of places in slots", map[string]any{"max": 50})
	if err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.SetPlacesResponse](s.server, s.st.SetEventPlaces(ctx, m.Org, m.Plan, int64(m.Event), int64(m.Task), int(n)))
}

func (s planSvc) ResetPlaces(ctx context.Context, req *connect.Request[v1.ResetPlacesRequest]) (*connect.Response[v1.ResetPlacesResponse], error) {
	m := req.Msg
	return done[v1.ResetPlacesResponse](s.server, s.st.SetEventPlaces(ctx, m.Org, m.Plan, int64(m.Event), int64(m.Task), 0))
}

func (s planSvc) SetPlacesMany(ctx context.Context, req *connect.Request[v1.SetPlacesManyRequest]) (*connect.Response[v1.SetPlacesManyResponse], error) {
	var places []store.Places
	if err := in(req.Msg, &struct {
		Places *[]store.Places `json:"places"`
	}{&places}); err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.SetPlacesManyResponse](s.server, s.st.SetPlacesMany(ctx, req.Msg.Org, req.Msg.Plan, places))
}

// ---- details rows ---------------------------------------------------------------------------------------------------

func (s planSvc) ListFields(ctx context.Context, req *connect.Request[v1.ListFieldsRequest]) (*connect.Response[v1.ListFieldsResponse], error) {
	v, err := s.st.ListFields(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListFieldsResponse](s.server, v, err)
}

func (s planSvc) AddField(ctx context.Context, req *connect.Request[v1.AddFieldRequest]) (*connect.Response[v1.AddFieldResponse], error) {
	f, err := s.st.AddField(ctx, req.Msg.Org, req.Msg.Plan, req.Msg.Name)
	return conv[v1.AddFieldResponse](s.server, map[string]any{"field": f}, err)
}

func (s planSvc) RenameField(ctx context.Context, req *connect.Request[v1.RenameFieldRequest]) (*connect.Response[v1.RenameFieldResponse], error) {
	m := req.Msg
	f, err := s.st.RenameField(ctx, m.Org, m.Plan, int64(m.Id), m.Name)
	return conv[v1.RenameFieldResponse](s.server, map[string]any{"field": f}, err)
}

func (s planSvc) MoveField(ctx context.Context, req *connect.Request[v1.MoveFieldRequest]) (*connect.Response[v1.MoveFieldResponse], error) {
	m := req.Msg
	pos, err := positionOf(m.Position)
	if err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.MoveFieldResponse](s.server, s.st.MoveField(ctx, m.Org, m.Plan, int64(m.Id), pos))
}

func (s planSvc) FreezeField(ctx context.Context, req *connect.Request[v1.FreezeFieldRequest]) (*connect.Response[v1.FreezeFieldResponse], error) {
	m := req.Msg
	frozen, err := required(m.Frozen, "frozen_missing", "send frozen: true or false", nil)
	if err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.FreezeFieldResponse](s.server, s.st.FreezeField(ctx, m.Org, m.Plan, int64(m.Id), frozen))
}

func (s planSvc) DeleteField(ctx context.Context, req *connect.Request[v1.DeleteFieldRequest]) (*connect.Response[v1.DeleteFieldResponse], error) {
	return done[v1.DeleteFieldResponse](s.server, s.st.DeleteField(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id)))
}

func (s planSvc) SetFieldValue(ctx context.Context, req *connect.Request[v1.SetFieldValueRequest]) (*connect.Response[v1.SetFieldValueResponse], error) {
	m := req.Msg
	return done[v1.SetFieldValueResponse](s.server, s.st.SetFieldValue(ctx, m.Org, m.Plan, int64(m.Event), int64(m.Field), m.Value))
}

// ---- answers --------------------------------------------------------------------------------------------------------

func (s planSvc) ListAnswers(ctx context.Context, req *connect.Request[v1.ListAnswersRequest]) (*connect.Response[v1.ListAnswersResponse], error) {
	v, err := s.st.ListAnswers(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListAnswersResponse](s.server, v, err)
}

func (s planSvc) SetAvailability(ctx context.Context, req *connect.Request[v1.SetAvailabilityRequest]) (*connect.Response[v1.SetAvailabilityResponse], error) {
	m := req.Msg
	levels := make(map[int64]string, len(m.Levels))
	for k, v := range m.Levels {
		levels[int64(k)] = v
	}
	return done[v1.SetAvailabilityResponse](s.server, s.st.SetAvailability(ctx, m.Org, m.Plan, int64(m.Person), levels))
}

func (s planSvc) SetNote(ctx context.Context, req *connect.Request[v1.SetNoteRequest]) (*connect.Response[v1.SetNoteResponse], error) {
	m := req.Msg
	return done[v1.SetNoteResponse](s.server, s.st.SetAnswerNote(ctx, m.Org, m.Plan, int64(m.Event), int64(m.Person), m.Note))
}

func (s planSvc) SetAnswers(ctx context.Context, req *connect.Request[v1.SetAnswersRequest]) (*connect.Response[v1.SetAnswersResponse], error) {
	var answers []store.Answer
	if err := in(req.Msg, &struct {
		Answers *[]store.Answer `json:"answers"`
	}{&answers}); err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.SetAnswersResponse](s.server, s.st.SetAnswers(ctx, req.Msg.Org, req.Msg.Plan, answers))
}
