package evalostic

import (
	"math/rand"
	"strings"
	"testing"
)

// fuzzStrings are used in generated conditions, every combination of them is matched against the
// conditions and compared with the reference implementation of the Condition types in evalostic_test.go.
// They overlap, so strings have to be found inside other strings and across the boundaries of joined strings.
var fuzzStrings = []string{"a", "ab", "ba", "c", "cc", "d"}

// fuzzMaxDepth keeps the generated conditions small, so that building and checking each of them against all
// inputs stays cheap. These tests check correctness only, large conditions are covered by TestLargeConditions.
const fuzzMaxDepth = 3

// fuzzInputs contains all combinations of fuzzStrings, joined with and without a separator and in
// lower, upper and mixed case
var fuzzInputs = func() []string {
	var res []string
	unique := make(map[string]struct{})
	for mask := 0; mask < 1<<len(fuzzStrings); mask++ {
		var parts []string
		for i, str := range fuzzStrings {
			if mask&(1<<i) != 0 {
				parts = append(parts, str)
			}
		}
		for _, sep := range []string{"", " "} {
			input := strings.Join(parts, sep)
			for _, variant := range []string{input, strings.ToUpper(input), mixedCase(input)} {
				if _, ok := unique[variant]; !ok {
					unique[variant] = struct{}{}
					res = append(res, variant)
				}
			}
		}
	}
	return res
}()

// mixedCase converts every second character of a lower case string to upper case
func mixedCase(s string) string {
	b := []byte(s)
	for i := 1; i < len(b); i += 2 {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

// assertMatchesReference compiles all conditions and compares their matches with the reference implementation,
// a nil condition is passed as empty condition
func assertMatchesReference(t *testing.T, conds []Condition) {
	t.Helper()
	strs := make([]string, len(conds))
	for i, cond := range conds {
		if cond != nil {
			strs[i] = cond.ToCondition()
		}
	}
	e, err := New(strs)
	if err != nil {
		t.Fatalf("parse %q: %s", strs, err)
	}
	for _, input := range fuzzInputs {
		var expected []int
		for i, cond := range conds {
			if cond != nil && cond.Match(input) {
				expected = append(expected, i)
			}
		}
		if actual := e.Match(input); !sameIntegers(actual, expected) {
			t.Fatalf("wrong matches for conditions %q and string %q: expected %v, got %v", strs, input, expected, actual)
		}
	}
}

// randomConditionTree generates a random condition, NOT is not nested directly because "NOT NOT" can not be parsed
func randomConditionTree(r *rand.Rand, depth int, allowNot bool) Condition {
	if depth == 0 || r.Intn(4) == 0 {
		return val(fuzzStrings[r.Intn(len(fuzzStrings))])
	}
	switch r.Intn(5) {
	case 0:
		if allowNot {
			return not(randomConditionTree(r, depth-1, false))
		}
		return val(fuzzStrings[r.Intn(len(fuzzStrings))])
	case 1, 2:
		return and(randomConditionTrees(r, depth-1)...)
	default:
		return or(randomConditionTrees(r, depth-1)...)
	}
}

func randomConditionTrees(r *rand.Rand, depth int) []Condition {
	res := make([]Condition, 2+r.Intn(2))
	for i := range res {
		res[i] = randomConditionTree(r, depth, true)
	}
	return res
}

func TestMatchAgainstReference(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	for i := 0; i < 2000; i++ {
		conds := make([]Condition, 1+r.Intn(5))
		for j := range conds {
			if r.Intn(10) > 0 { // keep some conditions empty
				conds[j] = randomConditionTree(r, fuzzMaxDepth, true)
			}
		}
		assertMatchesReference(t, conds)
	}
}

// nextByte consumes the next byte of the fuzzer data, 0 if there is none left
func nextByte(data *[]byte) int {
	if len(*data) == 0 {
		return 0
	}
	b := (*data)[0]
	*data = (*data)[1:]
	return int(b)
}

// decodeConditions builds 1 to 5 conditions from fuzzer data, some of them empty like in TestMatchAgainstReference
func decodeConditions(data *[]byte) []Condition {
	conds := make([]Condition, 1+nextByte(data)%5)
	for i := range conds {
		if nextByte(data)%8 != 7 { // keep some conditions empty
			conds[i] = decodeConditionTree(data, fuzzMaxDepth, true)
		}
	}
	return conds
}

// decodeConditionTree builds a condition from fuzzer data, every byte selects the next node and its parameter
func decodeConditionTree(data *[]byte, depth int, allowNot bool) Condition {
	b := nextByte(data)
	op, param := b%4, b/4
	if depth == 0 || op == 0 || (op == 1 && !allowNot) {
		return val(fuzzStrings[param%len(fuzzStrings)])
	}
	if op == 1 {
		return not(decodeConditionTree(data, depth-1, false))
	}
	subs := make([]Condition, 2+param%2)
	for i := range subs {
		subs[i] = decodeConditionTree(data, depth-1, true)
	}
	if op == 2 {
		return and(subs...)
	}
	return or(subs...)
}

func FuzzMatch(f *testing.F) {
	// "a"
	f.Add([]byte{})
	// NOT ("ab" OR "ba") AND "c", "", "a" OR "cc" OR "d"
	f.Add([]byte{0x02, 0x00, 0x02, 0x05, 0x03, 0x04, 0x08, 0x0c, 0x07, 0x00, 0x07, 0x00, 0x10, 0x14})
	f.Fuzz(func(t *testing.T, data []byte) {
		assertMatchesReference(t, decodeConditions(&data))
	})
}
