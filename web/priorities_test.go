package web

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// «What matters most» sets groups of weights to fixed values; Normal must be the defaults, or a plan that never changed
// anything would show «Custom», and every level must be within the weight's limits.
func TestPrioritiesNormalIsTheDefaults(t *testing.T) {
	src := pageSource(t)
	block := regexp.MustCompile(`(?s)const PRIORITIES = \[(.*?)\n\];`).FindSubmatch(src)
	if block == nil {
		t.Fatal("PRIORITIES not found in web/src")
	}
	specs := map[string]store.WeightSpec{}
	for _, s := range store.WeightSpecs {
		specs[s.Key] = s
	}
	level := regexp.MustCompile(`(less|normal|more): \{([^}]*)\}`)
	pair := regexp.MustCompile(`'?([\w.]+)'?: (-?[\d.]+)`)
	normals := 0
	for _, m := range level.FindAllSubmatch(block[1], -1) {
		for _, p := range pair.FindAllSubmatch(m[2], -1) {
			key, v := string(p[1]), string(p[2])
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				t.Fatalf("%s %s: %v", m[1], key, err)
			}
			spec, ok := specs[key]
			if !ok {
				t.Errorf("%s: %s is not a weight", m[1], key)
				continue
			}
			if n < spec.Min || n > spec.Max {
				t.Errorf("%s: %s = %v is outside %v..%v", m[1], key, n, spec.Min, spec.Max)
			}
			if string(m[1]) == "normal" {
				normals++
				if n != spec.Default {
					t.Errorf("normal: %s = %v, the default is %v", key, n, spec.Default)
				}
			}
		}
	}
	if normals == 0 {
		t.Fatal("no Normal values found")
	}
}
