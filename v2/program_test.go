package evalostic

import (
	"sort"
	"testing"
)

func TestIndexStrings(t *testing.T) {
	// "common" is contained in many conditions, "rare" and "other" only in one
	strs := []string{"common", "rare", "other"}
	stringIndices := map[string]int{"common": 0, "rare": 1, "other": 2}
	conditionCounts := []int{100, 1, 1}
	for _, test := range []struct {
		condition   string
		expected    []string
		emptyResult bool
	}{
		{`"common" AND "rare"`, []string{"rare"}, false},
		{`"common" OR "rare"`, []string{"common", "rare"}, false},
		{`"common" AND NOT "rare"`, []string{"common"}, false},
		{`NOT "common" AND NOT "rare"`, []string{"common", "rare"}, true},
		{`NOT "common" OR NOT "rare"`, []string{"rare"}, true},
		{`NOT ("common" AND "rare")`, []string{"rare"}, true},
		{`NOT ("common" OR "rare")`, []string{"common", "rare"}, true},
		{`"common" AND ("rare" OR "other")`, []string{"other", "rare"}, false},
		{`"common" AND "rare" AND "other"`, []string{"rare"}, false},
	} {
		root, err := parseCondition(test.condition)
		if err != nil {
			t.Fatal(err)
		}
		var p program
		indices, _, emptyResult := p.indexStrings(p.compile(root, stringIndices), conditionCounts)
		var actual []string
		for _, i := range indices {
			actual = append(actual, strs[i])
		}
		sort.Strings(actual)
		if !sameStrings(actual, test.expected) || emptyResult != test.emptyResult {
			t.Errorf("%s: expected %v (empty result %t), got %v (empty result %t)", test.condition, test.expected, test.emptyResult, actual, emptyResult)
		}
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
