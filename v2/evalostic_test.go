package evalostic

import (
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type Condition interface {
	Match(str string) bool
	GetStrings(into map[string]struct{})
	ToCondition() string
}

type NOT struct{ Sub Condition }

func (n NOT) ToCondition() string {
	if isANDorOR(n.Sub) {
		return fmt.Sprintf("NOT (%s)", n.Sub.ToCondition())
	}
	return fmt.Sprintf("NOT %s", n.Sub.ToCondition())
}

func (n NOT) GetStrings(into map[string]struct{}) {
	n.Sub.GetStrings(into)
}

func (n NOT) Match(str string) bool {
	return !n.Sub.Match(str)
}

type AND struct {
	Sub []Condition
}

func isANDorOR(sub Condition) bool {
	_, isAND := sub.(AND)
	_, isOR := sub.(OR)
	return isAND || isOR
}

func (a AND) GetStrings(into map[string]struct{}) {
	for _, sub := range a.Sub {
		sub.GetStrings(into)
	}
}

func (a AND) ToCondition() string {
	res := make([]string, len(a.Sub))
	for i, sub := range a.Sub {
		if isANDorOR(sub) {
			res[i] = fmt.Sprintf("(%s)", sub.ToCondition())
		} else {
			res[i] = sub.ToCondition()
		}
	}
	return fmt.Sprintf("%s", strings.Join(res, " AND "))
}

func (a AND) Match(str string) bool {
	for _, sub := range a.Sub {
		if !sub.Match(str) {
			return false
		}
	}
	return true
}

type OR struct {
	Sub []Condition
}

func (a OR) ToCondition() string {
	res := make([]string, len(a.Sub))
	for i, sub := range a.Sub {
		if isANDorOR(sub) {
			res[i] = fmt.Sprintf("(%s)", sub.ToCondition())
		} else {
			res[i] = sub.ToCondition()
		}
	}
	return fmt.Sprintf("%s", strings.Join(res, " OR "))
}
func (a OR) GetStrings(into map[string]struct{}) {
	for _, sub := range a.Sub {
		sub.GetStrings(into)
	}
}
func or(sub ...Condition) Condition  { return OR{Sub: sub} }
func and(sub ...Condition) Condition { return AND{Sub: sub} }
func not(sub Condition) Condition    { return NOT{Sub: sub} }
func val(str string) Condition       { return VALUE(str) }

func (a OR) Match(str string) bool {
	for _, sub := range a.Sub {
		if sub.Match(str) {
			return true
		}
	}
	return len(a.Sub) == 0
}

type VALUE string // Case Sensitive

func (v VALUE) Match(str string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(string(v)))
}

func (v VALUE) ToCondition() string {
	return strconv.Quote(string(v))
}

func (v VALUE) GetStrings(into map[string]struct{}) {
	into[string(v)] = struct{}{}
}

// TheTestConditions is used to test conditions in a tree structure against conditions that were
// parsed with evalostic. It will create a big Condition Comparison Table.
// Rules for the condition below:
// 1.) Do not use upper case strings, they will be automatically added in the tests
// 2.) Do not use more than 10 different strings in the complete conditions
var TheTestConditions = []Condition{
	// START

	val("a"),

	or(
		val("a"),
		val("b"),
	),

	and(
		val("a"),
		val("b"),
	),

	not(
		val("a"),
	),

	not(
		and(
			val("a"),
			val("b"),
		),
	),

	and(
		or(
			val("a"),
			val("b"),
		),
		or(
			val("c"),
			val("d"),
		),
	),

	or(
		and(
			val("a"),
			val("b"),
		),
		and(
			val("c"),
			val("d"),
		),
	),

	and(
		val("a"),
		val("b"),
		or(
			val("c"),
			val("d"),
			val("e"),
		),
	),

	and(
		not(
			val("a"),
		),
		not(
			val("b"),
		),
		or(
			val("c"),
			val("d"),
			val("e"),
		),
	),

	and(
		val("a"),
		val("b"),
		not(
			or(
				val("c"),
				val("d"),
				val("e"),
			),
		),
	),

	and(
		not(
			val("a"),
		),
		not(
			val("b"),
		),
		not(
			or(
				val("c"),
				val("d"),
				val("e"),
			),
		),
	),

	// regression: the and paths of NOT "c" AND (NOT "c" OR NOT "b") shared their backing array,
	// so one path was overwritten and strings containing "b" did not match anymore
	not(
		or(
			or(
				val("d"),
				val("f"),
			),
			val("c"),
			and(
				val("c"),
				val("b"),
			),
		),
	),

	// END
}

func TestEvalosticAgainstStringContains(t *testing.T) {
	fmt.Printf("%100s | %25s | %20s | %20s\n", "Condition", "Test String", "String Contains", "Evalostic Matcher")
	for _, cond := range TheTestConditions {
		// collect all strings of the condition that will be used to check all combinations against the strings
		strMap := make(map[string]struct{})
		cond.GetStrings(strMap)
		var allStrings []string
		for str := range strMap {
			allStrings = append(allStrings, str, strings.ToUpper(str))
		}
		if len(allStrings) > 20 {
			t.Fatalf("too many string exported, this would lead to memory issues")
		}
		sort.Strings(allStrings)
		// build the condition string that will be compared to the string contains method
		condition := cond.ToCondition()
		ev, err := New([]string{condition})
		if err != nil {
			t.Fatalf("parse %s: %s", condition, err)
		}
		// prepare the two matchers
		matcher1 := cond.Match
		matcher2 := func(str string) bool {
			return len(ev.Match(str)) > 0
		}
		compareStrings := []string{""}
		for _, str := range allStrings {
			deepCopy := make([]string, len(compareStrings), len(compareStrings)*2)
			copy(deepCopy, compareStrings)
			for _, compareString := range compareStrings {
				deepCopy = append(deepCopy, compareString+str)
			}
			compareStrings = deepCopy
		}
		for _, str := range compareStrings {
			matches1 := matcher1(str)
			matches2 := matcher2(str)
			fmt.Printf("%100s | %25s | %20s | %20s\n", condition, strconv.Quote(str), strconv.FormatBool(matches1), strconv.FormatBool(matches2))
			if matches1 != matches2 {
				t.Errorf("wrong behaviour for condition %s string %q string contains %t evalostic %t", condition, str, matches1, matches2)
			}
		}
	}
}

func assertTrue(t *testing.T, b bool) {
	_, f, l, _ := runtime.Caller(1)
	if !b {
		t.Fatalf("assertion failed in %s:%d", f, l)
	}
}

func sameIntegers(a []int, b []int) bool {
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

func TestEvalostic(t *testing.T) {
	e, err := New([]string{
		`"foo" OR "bar"`,
		`"baz" AND "qux"`,
		`("a" OR "b") AND ("c" OR "d")`,
		`"1" AND NOT "2"`,
		`"1" AND NOT "2"`,
	})
	assertTrue(t, err == nil)
	assertTrue(t, sameIntegers(e.Match("foo"), []int{0}))
	assertTrue(t, sameIntegers(e.Match("bar"), []int{0}))
	assertTrue(t, sameIntegers(e.Match("foo bar"), []int{0}))
	assertTrue(t, sameIntegers(e.Match("baz"), []int{}))
	assertTrue(t, sameIntegers(e.Match("baz qux"), []int{1}))
	assertTrue(t, sameIntegers(e.Match("qux baz"), []int{1}))
	assertTrue(t, sameIntegers(e.Match("ab"), []int{}))
	assertTrue(t, sameIntegers(e.Match("ac"), []int{2}))
	assertTrue(t, sameIntegers(e.Match("ad"), []int{2}))
	assertTrue(t, sameIntegers(e.Match("bc"), []int{2}))
	assertTrue(t, sameIntegers(e.Match("bd"), []int{2}))
	assertTrue(t, sameIntegers(e.Match("cd"), []int{}))
	assertTrue(t, sameIntegers(e.Match("abcd"), []int{2}))
	assertTrue(t, sameIntegers(e.Match("1"), []int{3, 4}))
	assertTrue(t, sameIntegers(e.Match("2"), []int{}))
	assertTrue(t, sameIntegers(e.Match("12"), []int{}))
}

// largeConditions returns ("a00" OR "b00") AND ("a01" OR "b01") AND ... and "x" AND NOT (("a00" AND "b00") OR ...)
// with n groups, the disjunctive normal form of both conditions has 2^n terms
func largeConditions(n int) []string {
	orGroups, andGroups := make([]string, n), make([]string, n)
	for i := 0; i < n; i++ {
		orGroups[i] = fmt.Sprintf(`("a%02d" OR "b%02d")`, i, i)
		andGroups[i] = fmt.Sprintf(`("a%02d" AND "b%02d")`, i, i)
	}
	return []string{strings.Join(orGroups, " AND "), `"x" AND NOT (` + strings.Join(andGroups, " OR ") + `)`}
}

// groupStrings returns the strings of the groups [from, to) of largeConditions with the given prefix
func groupStrings(prefix string, from, to int) string {
	var res []string
	for i := from; i < to; i++ {
		res = append(res, fmt.Sprintf("%s%02d", prefix, i))
	}
	return strings.Join(res, " ")
}

// TestLargeConditions checks conditions that ran out of memory with ~24 groups, when they were still converted to a
// disjunctive normal form
func TestLargeConditions(t *testing.T) {
	e, err := New(largeConditions(64))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		input    string
		expected []int
	}{
		{"all a", "x " + groupStrings("a", 0, 64), []int{0, 1}},
		{"last group missing", "x " + groupStrings("a", 0, 63), []int{1}},
		{"last group with b", "x " + groupStrings("a", 0, 63) + " b63", []int{0, 1}},
		{"all a and b", "x " + groupStrings("a", 0, 64) + " " + groupStrings("b", 0, 64), []int{0}},
		{"single group", "x a42 b42", []int{}},
	} {
		if actual := e.Match(test.input); !sameIntegers(actual, test.expected) {
			t.Errorf("%s: expected %v, got %v", test.name, test.expected, actual)
		}
	}
}

// TestMatchConcurrently calls Match from several goroutines, run it with -race to find shared state
func TestMatchConcurrently(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	conds := make([]string, 200)
	for i := range conds {
		conds[i] = randomConditionTree(r, fuzzMaxDepth, true).ToCondition()
	}
	e, err := New(conds)
	if err != nil {
		t.Fatal(err)
	}
	expected := make([][]int, len(fuzzInputs))
	for i, input := range fuzzInputs {
		expected[i] = e.Match(input)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range fuzzInputs {
				i := (j + g*len(fuzzInputs)/8) % len(fuzzInputs) // every goroutine starts at another input
				if actual := e.Match(fuzzInputs[i]); !sameIntegers(actual, expected[i]) {
					t.Errorf("goroutine %d, string %q: expected %v, got %v", g, fuzzInputs[i], expected[i], actual)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func ExampleEvalostic_Match() {
	e, err := New([]string{
		`"foo" OR "bar"`,
		`NOT "foo" AND ("bar" OR "baz")`,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(e.Match("foo"))
	fmt.Println(e.Match("bar"))
	fmt.Println(e.Match("foobar"))
	fmt.Println(e.Match("baz"))
	fmt.Println(e.Match("qux"))
	// Output:
	// [0]
	// [0 1]
	// [0]
	// [1]
	// []
}

func ExampleEvalostic_Match_negative() {
	e, err := New([]string{
		`NOT "foo" AND NOT "bar"`,
		`NOT ("foo" AND "bar" AND "baz")`,
		`"foo" OR NOT "baz"`,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(e.Match("foo"))
	fmt.Println(e.Match("bar"))
	fmt.Println(e.Match("baz"))
	fmt.Println(e.Match("foo bar"))
	fmt.Println(e.Match("foo bar baz"))
	fmt.Println(e.Match("qux"))
	// Output:
	// [1 2]
	// [1 2]
	// [0 1]
	// [1 2]
	// [2]
	// [0 1 2]
}

func ExampleEvalostic_Match_caseInsensitive() {
	e, err := New([]string{
		`"FOO"i AND "bar"`,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(e.Match("foo bar"))
	fmt.Println(e.Match("FoO BaR"))
	fmt.Println(e.Match("FOO BAR"))
	fmt.Println(e.Match("FoO bar"))
	// Output:
	// [0]
	// [0]
	// [0]
	// [0]
}

var ev *Evalostic

var validCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// rng is used instead of the global source, because rand.Seed() is a no-op since Go 1.24
var rng = rand.New(rand.NewSource(0))

func randomString(chars int) string {
	res := make([]byte, chars)
	for i := 0; i < chars; i++ {
		res[i] = validCharacters[rng.Intn(len(validCharacters))]
	}
	return string(res)
}

func randomCondition(maxConcatenations int) string {
	var cond string
	if rng.Intn(5) == 0 {
		cond += "NOT "
	}
	if cond != "" && !strings.HasSuffix(cond, " ") {
		cond += " "
	}
	concatenations := rng.Intn(maxConcatenations)
	if concatenations == 0 {
		cond += strconv.Quote(randomString(3 + rng.Intn(20)))
		if rng.Intn(2) == 0 {
			cond += "i"
		}
		return cond
	}
	cond += "("
	for i := 0; i < concatenations+1; i++ {
		if i > 0 {
			cond += []string{" AND ", " OR "}[rng.Intn(2)]
		}
		cond += randomCondition(maxConcatenations - 1)
	}
	cond += ")"
	return cond
}

func Example_randomString() {
	rng.Seed(0)
	fmt.Println(randomString(10))
	fmt.Println(randomString(10))
	// Output:
	// mUNERA9rI2
	// cvTK4UHomc
}

func Example_randomCondition() {
	rng.Seed(0)
	fmt.Println(randomCondition(3))
	fmt.Println(randomCondition(3))
	fmt.Println(randomCondition(3))
	fmt.Println(randomCondition(3))
	fmt.Println(randomCondition(3))
	fmt.Println(randomCondition(3))
	// Output:
	// "ERA9rI2cvTK4UHom"i
	// NOT "QvymkzADm"
	// ("HwxmE4tL20SrW"i AND (NOT "U86R7wIBbUt9RwI9U" OR "aWsz0l"i) AND "egogMR6spJPZHaPT0w4"i)
	// "nRPswXn0i6jt3PARUDb0oU"
	// NOT "aT3LnXs1oH7gq"i
	// (NOT ("K4k"i OR NOT "UcTwCQmwFJPivbFQtOWS") OR NOT "8bmfKkaauLI"i)
}

func BenchmarkNew(b *testing.B) {
	var err error
	for i := 0; i < b.N; i++ {
		ev, err = New([]string{`"foo" AND ("bar" OR NOT ("baz" AND NOT "qux"))`})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkNewN(b *testing.B, n int) {
	rng.Seed(0)
	conds := make([]string, n)
	for i := 0; i < n; i++ {
		conds[i] = randomCondition(3)
		if i < 10 {
			b.Log(conds[i])
		}
		if i == 10 {
			b.Logf("[skipping remaining %d conditions in outout]", n-10)
		}
	}
	b.ResetTimer()
	var err error
	for i := 0; i < b.N; i++ {
		ev, err = New(conds)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNew_10(b *testing.B)     { benchmarkNewN(b, 10) }
func BenchmarkNew_100(b *testing.B)    { benchmarkNewN(b, 100) }
func BenchmarkNew_1000(b *testing.B)   { benchmarkNewN(b, 1000) }
func BenchmarkNew_10000(b *testing.B)  { benchmarkNewN(b, 10000) }
func BenchmarkNew_100000(b *testing.B) { benchmarkNewN(b, 100000) }

var matches []int

func benchmarkMatchN(b *testing.B, n int) {
	rng.Seed(0)
	conds := make([]string, n)
	for i := 0; i < n; i++ {
		conds[i] = randomCondition(3)
	}
	ev, err := New(conds)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches = ev.Match("ERA9rI2cvTK4UHomQvymkzADmHwxmE4tL20SrWU86R7wIBbUt9RwI9UaWsz0legogMR6spJPZHaPT0w4n" +
			"RPswXn0i6jt3PARUDb0oUaT3LnXs1oH7gqK4kUcTwCQmwFJPivbFQtOWS8bmfKkaauLIHID5QOEwEYos1y7eNI1Had7lItaey9Y" +
			"WwoRhgmpWQc9DYV9uRIl8ILMGrvME4e8vdKlJUpEMrZqXlqPLn0eqyWgWIyTrJVVYSzaAAuZrNuNtZCN")
	}
	if len(matches) > 20 {
		b.Logf("matches: %+v...+%d matches", matches[:20], len(matches)-20)
	} else {
		b.Logf("matches: %+v", matches)
	}
}

func BenchmarkEvalostic_Match_10(b *testing.B)     { benchmarkMatchN(b, 10) }
func BenchmarkEvalostic_Match_100(b *testing.B)    { benchmarkMatchN(b, 100) }
func BenchmarkEvalostic_Match_1000(b *testing.B)   { benchmarkMatchN(b, 1000) }
func BenchmarkEvalostic_Match_10000(b *testing.B)  { benchmarkMatchN(b, 10000) }
func BenchmarkEvalostic_Match_100000(b *testing.B) { benchmarkMatchN(b, 100000) }

func BenchmarkNew_LargeConditions(b *testing.B) {
	conds := largeConditions(64)
	var err error
	for i := 0; i < b.N; i++ {
		ev, err = New(conds)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvalostic_Match_LargeConditions(b *testing.B) {
	e, err := New(largeConditions(64))
	if err != nil {
		b.Fatal(err)
	}
	s := "x " + groupStrings("a", 0, 64) + " " + groupStrings("b", 0, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches = e.Match(s)
	}
}

// BenchmarkEvalostic_Match_CommonString matches many conditions that share a common string against a string that
// contains only the common string
func BenchmarkEvalostic_Match_CommonString(b *testing.B) {
	conds := make([]string, 100000)
	for i := range conds {
		conds[i] = fmt.Sprintf(`"\\windows\\" AND "tool%d.exe"`, i)
	}
	e, err := New(conds)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches = e.Match(`C:\Windows\System32\cmd.exe`)
	}
}
