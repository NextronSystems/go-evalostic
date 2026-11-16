package evalostic

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Codehardt/go-ahocorasick"
)

// Evalostic is a matcher that can apply multiple conditions on a string with some performance optimizations.
// All strings of all conditions are searched at once with the Aho-Corasick algorithm. A condition is only evaluated
// if one of its index strings was found, which are the strings that can change its result (see indexStrings). All
// other conditions have the same result as for a string without any known strings, which is evaluated once in New().
type Evalostic struct {
	program      program
	roots        []int // root instruction of every condition
	emptyMatches []int // conditions that match strings without any of their strings, e.g. NOT "foo"
	ahoCorasick  ahocorasick.AhoCorasick
	mapping      [][]int // which string triggers the evaluation of which condition
	matchStates  *sync.Pool
	orig         []node // original conditions for export
}

// matchState is the reusable scratch space of a Match call
type matchState struct {
	found     bitset // indices of the found strings
	evaluated bitset // indices of the already evaluated conditions
}

// New builds a new Evalostic matcher that compiles all conditions to one big rule set that can be applied to strings.
func New(conditions []string) (*Evalostic, error) {
	e := Evalostic{
		roots:       make([]int, len(conditions)),
		matchStates: new(sync.Pool),
	}
	var (
		allStrings      []string
		stringIndices   = make(map[string]int)
		conditionCounts []int // number of conditions that contain a string
		lastCondition   []int // last condition that contains a string
	)
	for i, condition := range conditions {
		if condition == "" {
			continue // allow empty conditions but ignore them
		}
		root, err := parseCondition(condition)
		if err != nil {
			return nil, fmt.Errorf("condition %d: %s", i, err)
		}
		e.orig = append(e.orig, root)
		condStrings, _ := extractStrings(root)
		for _, str := range condStrings {
			strI, ok := stringIndices[str]
			if !ok {
				strI = len(allStrings)
				stringIndices[str] = strI
				allStrings = append(allStrings, str)
				conditionCounts = append(conditionCounts, 0)
				lastCondition = append(lastCondition, -1)
			}
			if lastCondition[strI] != i { // a string can occur multiple times in a condition
				lastCondition[strI] = i
				conditionCounts[strI]++
			}
		}
		e.roots[i] = e.program.compile(root, stringIndices)
	}
	e.mapping = make([][]int, len(allStrings))
	for i, condition := range conditions {
		if condition == "" {
			continue
		}
		strs, _, emptyResult := e.program.indexStrings(e.roots[i], conditionCounts)
		for _, strI := range strs {
			if m := e.mapping[strI]; len(m) == 0 || m[len(m)-1] != i { // index strings can contain duplicates
				e.mapping[strI] = append(m, i)
			}
		}
		if emptyResult {
			e.emptyMatches = append(e.emptyMatches, i)
		}
	}
	if len(allStrings) > 0 {
		e.ahoCorasick = ahocorasick.New(allStrings)
	}
	numStrings, numConditions := len(allStrings), len(conditions)
	e.matchStates.New = func() interface{} {
		return &matchState{found: newBitset(numStrings), evaluated: newBitset(numConditions)}
	}
	return &e, nil
}

// Match returns all indices of conditions that match the provided string. It is safe for concurrent use.
func (e *Evalostic) Match(s string) (matchingConditions []int) {
	var foundStrings []int
	if e.ahoCorasick != nil {
		foundStrings = e.ahoCorasick.Match(strings.ToLower(s))
	}
	state := e.matchStates.Get().(*matchState)
	for _, strI := range foundStrings {
		state.found.set(strI)
	}
	for _, strI := range foundStrings {
		for _, cond := range e.mapping[strI] {
			if state.evaluated.has(cond) {
				continue
			}
			state.evaluated.set(cond)
			if e.program.eval(e.roots[cond], state.found) {
				matchingConditions = append(matchingConditions, cond)
			}
		}
	}
	for _, cond := range e.emptyMatches {
		if !state.evaluated.has(cond) {
			matchingConditions = append(matchingConditions, cond)
		}
	}
	for _, strI := range foundStrings {
		state.found.unset(strI)
		for _, cond := range e.mapping[strI] {
			state.evaluated.unset(cond)
		}
	}
	e.matchStates.Put(state)
	sort.Ints(matchingConditions)
	return
}
