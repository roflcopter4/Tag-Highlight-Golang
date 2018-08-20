package mpack

import (
	"math"
)

func Make_New(len uint, encode bool) *Object {
	var root Object
	if encode {
		root.flags = mFLAG_ENCODE | mFLAG_PACKED
	}
	root.packed = make([]byte, 0, 128)
	tmp := &root
	root.Encode_Array(&tmp, len)

	return &root
}

//========================================================================================

func (root *Object) Encode_Array(item **Object, size uint) {
	(*item).Data = make([]Object, size, size)
	(*item).Mtype = T_ARRAY

	switch {
	case size <= mARRAY_F_MAX:
		root.packed = append(root.packed, mMASK_ARRAY_F|byte(size))
	case size <= math.MaxUint16:
		root.packed = append(root.packed, mMASK_ARRAY_16)
		encode_uint16(&root.packed, uint16(size))
	case size <= math.MaxUint32:
		root.packed = append(root.packed, mMASK_ARRAY_32)
		encode_uint32(&root.packed, uint32(size))
	default:
		panic("Array size is too large to encode.")
	}
}

func (root *Object) Encode_Map(item **Object, size uint) {
	(*item).Data = make([]Map_Entry, size)
	(*item).Mtype = T_MAP

	switch {
	case size <= mARRAY_F_MAX:
		root.packed = append(root.packed, mMASK_MAP_F|byte(size))
	case size <= math.MaxUint16:
		root.packed = append(root.packed, mMASK_MAP_16)
		encode_uint16(&root.packed, uint16(size))
	case size <= math.MaxUint32:
		root.packed = append(root.packed, mMASK_MAP_32)
		encode_uint32(&root.packed, uint32(size))
	default:
		panic("Map size is too large to encode.")
	}
}

func (root *Object) Encode_Integer(item **Object, value int64) {
	if (root.flags & mFLAG_ENCODE) != 0 {
		(*item).Mtype = T_NUM
		(*item).Data = value
	}

	if value >= 0 {
		switch {
		case value <= 127:
			root.packed = append(root.packed, mMASK_POS_INT_F|byte(value))
		case value <= math.MaxUint8:
			root.packed = append(root.packed, mMASK_UINT_8, byte(value))
		case value <= math.MaxUint16:
			root.packed = append(root.packed, mMASK_UINT_16)
			encode_uint16(&root.packed, uint16(value))
		case value <= math.MaxUint32:
			root.packed = append(root.packed, mMASK_UINT_32)
			encode_uint32(&root.packed, uint32(value))
		case value <= math.MaxInt64:
			root.packed = append(root.packed, mMASK_UINT_64)
			encode_uint64(&root.packed, uint64(value))
		default:
			panic("Value is too large to encode.")
		}
	} else {
		switch {
		case value >= -31:
			root.packed = append(root.packed, mMASK_NEG_INT_F|byte(value))
		case value >= math.MinInt8:
			root.packed = append(root.packed, mMASK_INT_8, byte(value&0xFF))
		case value >= math.MinInt16:
			root.packed = append(root.packed, mMASK_INT_16)
			encode_int16(&root.packed, int16(value))
		case value <= math.MinInt32:
			root.packed = append(root.packed, mMASK_INT_32)
			encode_int32(&root.packed, int32(value))
		case value <= math.MinInt64:
			root.packed = append(root.packed, mMASK_INT_64)
			encode_int64(&root.packed, int64(value))
		default:
			panic("Value is too small to encode.")
		}
	}
}

func (root *Object) Encode_String(item **Object, str []byte) {
	if (root.flags & mFLAG_ENCODE) != 0 {
		(*item).Data = str
		(*item).Mtype = T_STRING
	}

	switch {
	case len(str) <= 31:
		root.packed = append(root.packed, mMASK_STR_F|byte(len(str)))
	case len(str) <= math.MaxUint8:
		root.packed = append(root.packed, mMASK_STR_8, byte(len(str)))
	case len(str) <= math.MaxUint16:
		root.packed = append(root.packed, mMASK_STR_16)
		encode_uint16(&root.packed, uint16(len(str)))
	case len(str) <= math.MaxUint32:
		root.packed = append(root.packed, mMASK_STR_32)
		encode_uint32(&root.packed, uint32(len(str)))
	default:
		panic("String is too large to encode.")
	}

	root.packed = append(root.packed, str...)
}

func (root *Object) Encode_Boolean(item **Object, value bool) {
	if (root.flags & mFLAG_ENCODE) != 0 {
		(*item).Data = value
		(*item).Mtype = T_BOOL
	}

	if value {
		root.packed = append(root.packed, mMASK_TRUE)
	} else {
		root.packed = append(root.packed, mMASK_FALSE)
	}
}

func (root *Object) Encode_Nil(item **Object) {
	if (root.flags & mFLAG_ENCODE) != 0 {
		(*item).Data = uint16(0)
		(*item).Mtype = T_NIL
	}

	root.packed = append(root.packed, mMASK_NIL)
}

//========================================================================================

func encode_uint16(str *[]byte, val uint16) {
	*str = append(*str, byte(val>>010), byte(val&0xFF))
}

func encode_uint32(str *[]byte, val uint32) {
	*str = append(*str,
		byte(val>>030), byte(val>>020), byte(val>>010), byte(val&0xFF))
}

func encode_uint64(str *[]byte, val uint64) {
	*str = append(*str,
		byte(val>>070), byte(val>>060), byte(val>>050),
		byte(val>>040), byte(val>>030), byte(val>>020),
		byte(val>>010), byte(val&0xFF))
}

func encode_int16(str *[]byte, val int16) {
	*str = append(*str, byte(val>>010), byte(val&0xFF))
}

func encode_int32(str *[]byte, val int32) {
	*str = append(*str,
		byte(val>>030), byte(val>>020), byte(val>>010), byte(val&0xFF))
}

func encode_int64(str *[]byte, val int64) {
	*str = append(*str,
		byte(val>>070), byte(val>>060), byte(val>>050),
		byte(val>>040), byte(val>>030), byte(val>>020),
		byte(val>>010), byte(val&0xFF))
}
