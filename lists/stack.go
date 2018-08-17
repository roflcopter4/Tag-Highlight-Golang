package lists

import (
	"math"
)

type Stack struct {
	stack  []interface{}
	nilret interface{}
	ctr    uint
}

//========================================================================================

func New_Stack(initlen int, nilret interface{}) *Stack {
	return &Stack{
		stack:  make([]interface{}, initlen),
		nilret: nilret,
		ctr:    0,
	}
}

func (stk *Stack) Push(val interface{}) {
	if stk.ctr >= uint(len(stk.stack)) {
		stk.stack = append(stk.stack,
			make([]interface{}, len(stk.stack))...)
	}

	stk.stack[stk.ctr] = val
	stk.ctr++
}

func (stk *Stack) Pop() interface{} {
	if stk.ctr == 0 {
		panic("Can't pop from empty stack.")
	}

	stk.ctr--
	ret := stk.stack[stk.ctr]
	stk.stack[stk.ctr] = nil

	return ret
}

func (stk Stack) Peek() interface{} {
	return stk.PeekAt(0)
}

func (stk Stack) PeekAt(offset int) interface{} {
	var (
		abs   = int64(math.Abs(float64(offset)))
		index = int64(stk.ctr) - abs - 1
	)

	if stk.ctr == 0 || index < 0 || index > int64(len(stk.stack)) {
		return stk.nilret
	}
	return stk.stack[index]
}

func (stk *Stack) Reset() {
	stk.ctr = 0
	stk.stack[0] = nil
}
