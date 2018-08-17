package lists

import (
	"math"
)

type Array struct {
	data []uint
	ctr  uint
}

//========================================================================================

func New_Array(len int) *Array {
	ret := Array{
		data: make([]uint, len),
		ctr:  0,
	}

	return &ret
}

func (arr Array) Get() uint {
	return arr.GetOffset(0)
}

func (arr Array) GetOffset(offset int) uint {
	var (
		abs   = int64(math.Abs(float64(offset)))
		index = int64(arr.ctr) - abs - 1
	)

	if index < 0 || index > int64(len(arr.data)) {
		return 0
	}
	return arr.data[index]
}

func (arr Array) GetInd(index int) uint {
	if index < 0 || index > len(arr.data) {
		return 0
	}
	return arr.data[index]
}
