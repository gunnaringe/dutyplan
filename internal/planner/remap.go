package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func init() { store.RunRemapper = remapRun }

// idToken is a task or person id as the planner sees them («t12», «p5»), also inside its texts («locked: p5 on t12 e3 …»).
var idToken = regexp.MustCompile(`\b([tp])(\d+)\b`)

// remapRun rewrites the ids of people and tasks in a run (migration 012 gave every plan its own). The hash is rewritten
// only if it was the hash of the input, so a run that was out of date stays so and one that was not is still current.
func remapRun(r store.RunJSON, people, tasks map[int64]int64) (store.RunJSON, error) {
	text := func(s string) string {
		return idToken.ReplaceAllStringFunc(s, func(tok string) string {
			m := people
			if tok[0] == 't' {
				m = tasks
			}
			n, _ := strconv.ParseInt(tok[1:], 10, 64)
			if to, ok := m[n]; ok {
				return tok[:1] + strconv.FormatInt(to, 10)
			}
			return tok
		})
	}
	id := func(m map[int64]int64, n int64) int64 {
		if to, ok := m[n]; ok {
			return to
		}
		return n
	}
	out := r
	var in solverInput
	if err := json.Unmarshal([]byte(r.Input), &in); err != nil {
		return r, fmt.Errorf("run input: %w", err)
	}
	current := hashOf(in) == r.Hash
	in = solverInput{}
	if err := json.Unmarshal([]byte(text(r.Input)), &in); err != nil {
		return r, fmt.Errorf("run input: %w", err)
	}
	data, err := json.Marshal(in)
	if err != nil {
		return r, err
	}
	out.Input = string(data)
	if current {
		out.Hash = hashOf(in)
	}

	var snap Snapshot
	if err := json.Unmarshal([]byte(r.Snapshot), &snap); err != nil {
		return r, fmt.Errorf("run snapshot: %w", err)
	}
	for i := range snap.People {
		snap.People[i].ID = id(people, snap.People[i].ID)
	}
	for i := range snap.Tasks {
		snap.Tasks[i].ID = id(tasks, snap.Tasks[i].ID)
	}
	for e, places := range snap.Places {
		moved := make(map[int64]int, len(places))
		for t, n := range places {
			moved[id(tasks, t)] = n
		}
		snap.Places[e] = moved
	}
	for i, k := range snap.Kept {
		snap.Kept[i] = text(k)
	}
	if data, err = json.Marshal(snap); err != nil {
		return r, err
	}
	out.Snapshot = string(data)
	if r.Output.Valid {
		out.Output.String = text(r.Output.String)
	}
	return out, nil
}

// hashOf is the hash BuildInput gives the input: without the time limit, seed and workers, which are not part of it.
func hashOf(in solverInput) string {
	if in.Params != nil {
		params := map[string]any{}
		for k, v := range in.Params {
			if k != "time_limit" && k != "seed" && k != "workers" {
				params[k] = v
			}
		}
		in.Params = params
		if len(params) == 0 {
			in.Params = nil
		}
	}
	data, _ := json.Marshal(in)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
