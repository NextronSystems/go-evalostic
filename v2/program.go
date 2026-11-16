package evalostic

type instructionType uint8

const (
	instructionTypeVAL instructionType = iota + 1
	instructionTypeNOT
	instructionTypeAND
	instructionTypeOR
)

// instruction is a node of a compiled condition, nested ANDs and ORs are flattened into one instruction
type instruction struct {
	instructionType instructionType
	arg             int // VAL: string index, NOT: operand, AND / OR: offset of the operands
	numOperands     int // AND / OR: number of operands
}

// program contains the compiled conditions. They are evaluated directly instead of being converted into a normal
// form, because the normal form grows exponentially for conditions like ("a" OR "b") AND ("c" OR "d") AND ...
type program struct {
	instructions []instruction
	operands     []int
}

// compile appends the instructions of a condition and returns the index of its root instruction
func (p *program) compile(n node, stringIndices map[string]int) int {
	switch v := n.(type) {
	case nodeVAL:
		p.instructions = append(p.instructions, instruction{instructionType: instructionTypeVAL, arg: stringIndices[v.nodeValue]})
	case nodeNOT:
		if not, ok := v.node.(nodeNOT); ok { // remove double negation
			return p.compile(not.node, stringIndices)
		}
		operand := p.compile(v.node, stringIndices)
		p.instructions = append(p.instructions, instruction{instructionType: instructionTypeNOT, arg: operand})
	case nodeAND:
		return p.compileOperands(instructionTypeAND, flattenAnd(v), stringIndices)
	case nodeOR:
		return p.compileOperands(instructionTypeOR, flattenOr(v), stringIndices)
	default:
		panic("unknown node type")
	}
	return len(p.instructions) - 1
}

func (p *program) compileOperands(t instructionType, nodes []node, stringIndices map[string]int) int {
	operands := make([]int, len(nodes))
	for i, n := range nodes {
		operands[i] = p.compile(n, stringIndices)
	}
	p.instructions = append(p.instructions, instruction{instructionType: t, arg: len(p.operands), numOperands: len(operands)})
	p.operands = append(p.operands, operands...)
	return len(p.instructions) - 1
}

// eval evaluates the instruction with index i, found contains the indices of all strings that were found
func (p *program) eval(i int, found bitset) bool {
	in := p.instructions[i]
	switch in.instructionType {
	case instructionTypeVAL:
		return found.has(in.arg)
	case instructionTypeNOT:
		return !p.eval(in.arg, found)
	case instructionTypeAND:
		for _, operand := range p.operands[in.arg : in.arg+in.numOperands] {
			if !p.eval(operand, found) {
				return false
			}
		}
		return true
	case instructionTypeOR:
		for _, operand := range p.operands[in.arg : in.arg+in.numOperands] {
			if p.eval(operand, found) {
				return true
			}
		}
		return false
	default:
		panic("unknown instruction type")
	}
}

// indexStrings returns strings of the instruction with index i, so that the instruction evaluates to emptyResult as
// long as none of these strings are found. Only these strings have to trigger an evaluation, the condition count of
// each string is used to keep the triggering strings rare. cost is the sum of their condition counts.
func (p *program) indexStrings(i int, conditionCounts []int) (strs []int, cost int, emptyResult bool) {
	in := p.instructions[i]
	switch in.instructionType {
	case instructionTypeVAL:
		return []int{in.arg}, conditionCounts[in.arg], false
	case instructionTypeNOT:
		strs, cost, emptyResult = p.indexStrings(in.arg, conditionCounts)
		return strs, cost, !emptyResult
	case instructionTypeAND, instructionTypeOR:
		// an operand that is false for AND or true for OR without its strings decides the result alone,
		// otherwise every operand can change the result
		decidingResult := in.instructionType == instructionTypeOR
		var allStrs, decidingStrs []int
		allCost, decidingCost := 0, -1
		for _, operand := range p.operands[in.arg : in.arg+in.numOperands] {
			operandStrs, operandCost, operandResult := p.indexStrings(operand, conditionCounts)
			if operandResult == decidingResult && (decidingCost < 0 || operandCost < decidingCost) {
				decidingStrs, decidingCost = operandStrs, operandCost
			}
			allStrs, allCost = append(allStrs, operandStrs...), allCost+operandCost
		}
		if decidingCost >= 0 {
			return decidingStrs, decidingCost, decidingResult
		}
		return allStrs, allCost, !decidingResult
	default:
		panic("unknown instruction type")
	}
}

type bitset []uint64

func newBitset(size int) bitset { return make(bitset, (size+63)/64) }

func (b bitset) has(i int) bool { return b[i/64]&(1<<(uint(i)%64)) != 0 }

func (b bitset) set(i int) { b[i/64] |= 1 << (uint(i) % 64) }

func (b bitset) unset(i int) { b[i/64] &^= 1 << (uint(i) % 64) }
