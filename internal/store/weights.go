package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// WeightSpec describes one setting of the planner's weighting: its default (the planner's own, checked against the
// planner by a test), and the range it may be set within. Keys with a dot («w_task.good») are nested in the planner's input.
type WeightSpec struct {
	Key     string   `json:"key"`
	Kind    string   `json:"kind"` // number | choice
	Default any      `json:"default"`
	Min     float64  `json:"min"`
	Max     float64  `json:"max"`
	Options []string `json:"options,omitempty"` // for a choice
}

func num(key string, def, min, max float64) WeightSpec {
	return WeightSpec{Key: key, Kind: "number", Default: def, Min: min, Max: max}
}

// WeightSpecs is every weight a plan may change, in the order they are shown. Not here on purpose: what an empty place
// costs (it must come after everything else), stability and history (the app does not use them), and the neutral levels (zero).
var WeightSpecs = []WeightSpec{
	num("w_fair", 10, 0, 100),       // points per assignment a person is outside their share of all assignments
	num("w_fair_range", 10, 0, 100), // ... and for the worst deviation
	num("w_task_fair", 0, 0, 100),   // the same per task (equal share of each task, so unequal totals)
	num("w_task_range", 0, 0, 100),  // the worst deviation per task
	num("w_task.good", 2, 0, 100),   // someone gets a task they said «good» to
	num("w_task.poor", -4, -100, 0), // ... or «dårlig»
	num("w_date.good", 3, 0, 100),   // the same for an event
	num("w_date.poor", -6, -100, 0),
	num("w_combo.preferred", 6, 0, 100), // two tasks the same day that «bør kombineres»
	num("w_combo.practical", 2, 0, 100),
	num("w_combo.impractical", -5, -100, 0),
	{Key: "default_combination", Kind: "choice", Default: "impractical", Options: []string{"cannot", "impractical", "practical", "preferred"}},
}

func weightSpec(key string) (WeightSpec, bool) {
	for _, s := range WeightSpecs {
		if s.Key == key {
			return s, true
		}
	}
	return WeightSpec{}, false
}

// cleanWeights checks every key and value and drops what equals the default, so a changed default reaches plans that never
// chose a value. Numbers may be float64 or int (JSON gives float64).
func cleanWeights(in map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for key, raw := range in {
		spec, ok := weightSpec(key)
		if !ok {
			return nil, fmt.Errorf("%w: unknown weight %q", ErrInvalid, key)
		}
		switch spec.Kind {
		case "choice":
			s, isStr := raw.(string)
			valid := false
			for _, o := range spec.Options {
				valid = valid || o == s
			}
			if !isStr || !valid {
				return nil, fmt.Errorf("%w: %s must be one of %s", ErrInvalid, key, strings.Join(spec.Options, ", "))
			}
			if s != spec.Default {
				out[key] = s
			}
		default:
			var v float64
			switch x := raw.(type) {
			case float64:
				v = x
			case int:
				v = float64(x)
			case int64:
				v = float64(x)
			default:
				return nil, fmt.Errorf("%w: %s must be a number", ErrInvalid, key)
			}
			if math.IsNaN(v) || math.IsInf(v, 0) || v < spec.Min || v > spec.Max {
				return nil, fmt.Errorf("%w: %s must be between %g and %g", ErrInvalid, key, spec.Min, spec.Max)
			}
			if v != spec.Default {
				out[key] = v
			}
		}
	}
	return out, nil
}

// effective fills in the defaults: what the planner will really use.
func effective(values map[string]any) map[string]any {
	out := map[string]any{}
	for _, s := range WeightSpecs {
		out[s.Key] = s.Default
		if v, ok := values[s.Key]; ok {
			out[s.Key] = v
		}
	}
	return out
}

func getWeights(ctx context.Context, q dbq, oid, pid int64) (map[string]any, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT weights FROM plan WHERE org_id = ? AND id = ?`, oid, pid).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("plan weights are not readable: %w", err)
	}
	return values, nil
}

// GetWeights returns what the plan has changed from the defaults.
func (s *Store) GetWeights(ctx context.Context, orgSlug, planSlug string) (map[string]any, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return nil, err
	}
	return getWeights(ctx, s.db, oid, pid)
}

// SetWeights replaces what the plan has changed from the defaults (an empty map goes back to the defaults) and
// returns the stored result. Unknown keys and values outside their range are refused, and so is a change that changes nothing.
func (s *Store) SetWeights(ctx context.Context, orgSlug, planSlug, actor string, values map[string]any) (map[string]any, error) {
	clean, err := cleanWeights(values)
	if err != nil {
		return nil, err
	}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		old, err := getWeights(ctx, tx, oid, pid)
		if err != nil {
			return logEntry{}, err
		}
		before, after := effective(old), effective(clean)
		var changes []string
		for _, spec := range WeightSpecs {
			if before[spec.Key] != after[spec.Key] {
				changes = append(changes, fmt.Sprintf("%s %v -> %v", spec.Key, before[spec.Key], after[spec.Key]))
			}
		}
		if len(changes) == 0 {
			return logEntry{}, fmt.Errorf("%w: nothing to change", ErrInvalid)
		}
		sort.Strings(changes)
		b, _ := json.Marshal(clean)
		if _, err := tx.ExecContext(ctx, `UPDATE plan SET weights = ? WHERE org_id = ? AND id = ?`, string(b), oid, pid); err != nil {
			return logEntry{}, err
		}
		detail := strings.Join(changes, "; ")
		if len(clean) == 0 {
			detail = "tilbake til standard (" + detail + ")"
		}
		return logEntry{orgID: oid, planID: &pid, action: "plan.weights", detail: clip(detail, 200)}, nil
	})
	if err != nil {
		return nil, err
	}
	return clean, nil
}
