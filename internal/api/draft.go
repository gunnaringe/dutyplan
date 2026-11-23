package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/gunnaringe/dutyplan/gen/dutyplan/v1"
	"github.com/gunnaringe/dutyplan/internal/i18n"
	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

// draftSvc: places set by hand, selections, locks, the weights and the planner's runs.
type draftSvc struct{ *server }

func (s draftSvc) ListCells(ctx context.Context, req *connect.Request[v1.ListCellsRequest]) (*connect.Response[v1.ListCellsResponse], error) {
	v, err := s.st.ListCells(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListCellsResponse](s.server, map[string]any{"cells": v}, err)
}

func (s draftSvc) AddCell(ctx context.Context, req *connect.Request[v1.AddCellRequest]) (*connect.Response[v1.AddCellResponse], error) {
	m := req.Msg
	c, err := s.st.AddCell(ctx, m.Org, m.Plan, store.NewCell{EventID: int64(m.Event), TaskID: int64(m.Task), PersonID: int64(m.Person),
		Label: m.Label, Fills: int(m.Fills), Locked: m.Locked})
	return conv[v1.AddCellResponse](s.server, map[string]any{"cell": c}, err)
}

func (s draftSvc) DeleteCell(ctx context.Context, req *connect.Request[v1.DeleteCellRequest]) (*connect.Response[v1.DeleteCellResponse], error) {
	return done[v1.DeleteCellResponse](s.server, s.st.DeleteCell(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id)))
}

func (s draftSvc) ListPools(ctx context.Context, req *connect.Request[v1.ListPoolsRequest]) (*connect.Response[v1.ListPoolsResponse], error) {
	v, err := s.st.ListPools(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListPoolsResponse](s.server, map[string]any{"pools": v}, err)
}

func (s draftSvc) SetPool(ctx context.Context, req *connect.Request[v1.SetPoolRequest]) (*connect.Response[v1.SetPoolResponse], error) {
	m := req.Msg
	people := make([]int64, len(m.People))
	for i, p := range m.People {
		people[i] = int64(p)
	}
	return done[v1.SetPoolResponse](s.server, s.st.SetPool(ctx, m.Org, m.Plan, int64(m.Event), int64(m.Task), people))
}

// SetLocks locks the planner's names so a new draft keeps them, or unlocks them.
func (s draftSvc) SetLocks(ctx context.Context, req *connect.Request[v1.SetLocksRequest]) (*connect.Response[v1.SetLocksResponse], error) {
	var locks []store.Lock
	if err := in(req.Msg, &struct {
		Locks *[]store.Lock `json:"locks"`
	}{&locks}); err != nil {
		return nil, fail(s.log, err)
	}
	return done[v1.SetLocksResponse](s.server, s.st.SetLocks(ctx, req.Msg.Org, req.Msg.Plan, locks, req.Msg.Locked))
}

// SetDraftAside: «Remove unlocked».
func (s draftSvc) SetDraftAside(ctx context.Context, req *connect.Request[v1.SetDraftAsideRequest]) (*connect.Response[v1.SetDraftAsideResponse], error) {
	return done[v1.SetDraftAsideResponse](s.server, s.st.SetDraftAside(ctx, req.Msg.Org, req.Msg.Plan))
}

// ClearDraftAside: undoing «Remove unlocked».
func (s draftSvc) ClearDraftAside(ctx context.Context, req *connect.Request[v1.ClearDraftAsideRequest]) (*connect.Response[v1.ClearDraftAsideResponse], error) {
	return done[v1.ClearDraftAsideResponse](s.server, s.st.ClearDraftAside(ctx, req.Msg.Org, req.Msg.Plan))
}

// ---- weights --------------------------------------------------------------------------------------------------------

func (s draftSvc) GetWeights(ctx context.Context, req *connect.Request[v1.GetWeightsRequest]) (*connect.Response[v1.GetWeightsResponse], error) {
	values, err := s.st.GetWeights(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.GetWeightsResponse](s.server, map[string]any{"specs": store.WeightSpecs, "values": orEmpty(values)}, err)
}

// SetWeights replaces what the plan has changed: a key left out goes back to the default, {} resets all.
func (s draftSvc) SetWeights(ctx context.Context, req *connect.Request[v1.SetWeightsRequest]) (*connect.Response[v1.SetWeightsResponse], error) {
	values, err := s.st.SetWeights(ctx, req.Msg.Org, req.Msg.Plan, orEmpty(req.Msg.Values.AsMap()))
	return conv[v1.SetWeightsResponse](s.server, map[string]any{"specs": store.WeightSpecs, "values": orEmpty(values)}, err)
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// ---- runs -----------------------------------------------------------------------------------------------------------

// StartRun makes a draft; the planner runs in the background, and the stream says when it ends.
func (s draftSvc) StartRun(ctx context.Context, req *connect.Request[v1.StartRunRequest]) (*connect.Response[v1.StartRunResponse], error) {
	id, err := s.planner.Start(ctx, req.Msg.Org, req.Msg.Plan, planner.Params{TimeLimit: int(req.Msg.TimeLimit)})
	if err != nil {
		return nil, fail(s.log, err)
	}
	return res(&v1.StartRunResponse{Id: int32(id)})
}

func (s draftSvc) ListRuns(ctx context.Context, req *connect.Request[v1.ListRunsRequest]) (*connect.Response[v1.ListRunsResponse], error) {
	v, err := s.planner.List(ctx, req.Msg.Org, req.Msg.Plan)
	return conv[v1.ListRunsResponse](s.server, map[string]any{"runs": v}, err)
}

// GetRun: a run with its places and explanations in the language asked for; id 0 is the newest run of the plan.
func (s draftSvc) GetRun(ctx context.Context, req *connect.Request[v1.GetRunRequest]) (*connect.Response[v1.GetRunResponse], error) {
	m := req.Msg
	id := int64(m.Id)
	if id == 0 {
		runs, err := s.planner.List(ctx, m.Org, m.Plan)
		if err != nil {
			return nil, fail(s.log, err)
		}
		if len(runs) == 0 {
			return nil, fail(s.log, store.ErrNotFound)
		}
		id = runs[0].ID
	}
	v, err := s.planner.View(ctx, m.Org, m.Plan, id, i18n.Parse(m.Lang))
	return conv[v1.GetRunResponse](s.server, v, err)
}

// PinRun pins a finished run as the plan's current plan.
func (s draftSvc) PinRun(ctx context.Context, req *connect.Request[v1.PinRunRequest]) (*connect.Response[v1.PinRunResponse], error) {
	return done[v1.PinRunResponse](s.server, s.st.PinRun(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), true))
}

func (s draftSvc) UnpinRun(ctx context.Context, req *connect.Request[v1.UnpinRunRequest]) (*connect.Response[v1.UnpinRunResponse], error) {
	return done[v1.UnpinRunResponse](s.server, s.st.PinRun(ctx, req.Msg.Org, req.Msg.Plan, int64(req.Msg.Id), false))
}
