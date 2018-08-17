package mpack

import (
	"log"
	"syscall"
)

const (
	grp_NIL = iota
	grp_BOOL
	grp_ARRAY
	grp_MAP
	grp_STRING
	grp_BIN
	grp_INT
	grp_UINT
	grp_PLINT
	grp_NLINT
	grp_EXT
)
const (
	m_NIL = iota
	m_TRUE
	m_FALSE
	m_ARRAY_F
	m_ARRAY_E
	m_ARRAY_16
	m_ARRAY_32
	m_FIXMAP_F
	m_FIXMAP_E
	m_MAP_16
	m_MAP_32
	m_FIXSTR_F
	m_FIXSTR_E
	m_STR_8
	m_STR_16
	m_STR_32
	m_BIN_8
	m_BIN_16
	m_BIN_32
	m_INT_8
	m_INT_16
	m_INT_32
	m_INT_64
	m_UINT_8
	m_UINT_16
	m_UINT_32
	m_UINT_64
	m_POS_INT_F
	m_POS_INT_E
	m_NEG_INT_F
	m_NEG_INT_E
	m_EXT_8
	m_EXT_16
	m_EXT_32
	m_EXT_F1
	m_EXT_F2
	m_EXT_F4
)
const errmsg string = "Error: default reached somehow."

type read_fn func(src *interface{}, dest []byte, nbytes uint)
type mpack_mask struct {
	group, mtype int
	fixed        bool
	val, shift   uint8
	repr         string
}

var m_masks = [29]mpack_mask{
	{grp_NIL, m_NIL, false, 0xC0, 0, "m_NIL"},
	{grp_BOOL, m_TRUE, false, 0xC3, 0, "m_TRUE"},
	{grp_BOOL, m_FALSE, false, 0xC2, 0, "m_FALSE"},
	{grp_STRING, m_STR_8, false, 0xD9, 0, "m_STR_8"},
	{grp_STRING, m_STR_16, false, 0xDA, 0, "m_STR_16"},
	{grp_STRING, m_STR_32, false, 0xDB, 0, "m_STR_32"},
	{grp_ARRAY, m_ARRAY_16, false, 0xDC, 0, "m_ARRAY_16"},
	{grp_ARRAY, m_ARRAY_32, false, 0xDD, 0, "m_ARRAY_32"},
	{grp_MAP, m_MAP_16, false, 0xDE, 0, "m_MAP_16"},
	{grp_MAP, m_MAP_32, false, 0xDF, 0, "m_MAP_32"},
	{grp_BIN, m_BIN_8, false, 0xC4, 0, "m_BIN_8"},
	{grp_BIN, m_BIN_16, false, 0xC5, 0, "m_BIN_16"},
	{grp_BIN, m_BIN_32, false, 0xC6, 0, "m_BIN_32"},
	{grp_INT, m_INT_8, false, 0xD0, 0, "m_INT_8"},
	{grp_INT, m_INT_16, false, 0xD1, 0, "m_INT_16"},
	{grp_INT, m_INT_32, false, 0xD2, 0, "m_INT_32"},
	{grp_INT, m_INT_64, false, 0xD3, 0, "m_INT_64"},
	{grp_UINT, m_UINT_8, false, 0xCC, 0, "m_UINT_8"},
	{grp_UINT, m_UINT_16, false, 0xCD, 0, "m_UINT_16"},
	{grp_UINT, m_UINT_32, false, 0xCE, 0, "m_UINT_32"},
	{grp_UINT, m_UINT_64, false, 0xCF, 0, "m_UINT_64"},
	{grp_EXT, m_EXT_F1, false, 0xD4, 0, "m_EXT_F1"},
	{grp_EXT, m_EXT_F2, false, 0xD5, 0, "m_EXT_F2"},
	{grp_EXT, m_EXT_F4, false, 0xD6, 0, "m_EXT_F4"},
	{grp_STRING, m_FIXSTR_F, true, 0xA0, 5, "m_FIXSTR_F"},
	{grp_ARRAY, m_ARRAY_F, true, 0x90, 4, "m_ARRAY_F"},
	{grp_MAP, m_FIXMAP_F, true, 0x80, 4, "m_FIXMAP_F"},
	{grp_PLINT, m_POS_INT_F, true, 0x00, 7, "m_POS_INT_F"},
	{grp_NLINT, m_NEG_INT_F, true, 0xE0, 5, "m_NEG_INT_F"},
}

//========================================================================================

func decode_int16(b []byte) int16 {
	return (int16(b[0]) << 010) | int16(b[1])
}

func decode_int32(b []byte) int32 {
	return ((int32(b[0]) << 030) | (int32(b[1]) << 020) |
		(int32(b[2]) << 010) | int32(b[3]))
}

func decode_int64(b []byte) int64 {
	return ((int64(b[0]) << 070) | (int64(b[1]) << 060) | (int64(b[2]) << 050) |
		(int64(b[3]) << 040) | (int64(b[4]) << 030) | (int64(b[5]) << 020) |
		(int64(b[6]) << 010) | int64(b[7]))
}

func decode_uint16(b []byte) uint16 {
	return ((uint16(b[0]) << 010) | uint16(b[1]))
}

func decode_uint32(b []byte) uint32 {
	return ((uint32(b[0]) << 030) | (uint32(b[1]) << 020) |
		(uint32(b[2]) << 010) | uint32(b[3]))
}

func decode_uint64(b []byte) uint64 {
	return ((uint64(b[0]) << 070) | (uint64(b[1]) << 060) | (uint64(b[2]) << 050) |
		(uint64(b[3]) << 040) | (uint64(b[4]) << 030) | (uint64(b[5]) << 020) |
		(uint64(b[6]) << 010) | uint64(b[7]))
}

//========================================================================================

func Decode_Stream(fd int) *Object {
	var tmp interface{} = fd
	return do_decode(
		func(src *interface{}, dest []byte, nbytes uint) {
			fd := (*src).(int)
			n, e := syscall.Read(fd, dest[:nbytes])
			if e != nil || uint(n) != nbytes {
				panic(e)
			}
		}, &tmp)
}

func (obj *Object) Decode(str []byte) *Object {
	var tmp interface{} = obj.GetPack()
	if tmp == nil {
		panic("Cannot decode non-packed object.")
	}
	return do_decode(
		func(src *interface{}, dest []byte, nbytes uint) {
			for i := uint(0); i < nbytes; i++ {
				dest[i] = ((*src).([]byte))[i]
			}
			*src = ((*src).([]byte))[nbytes:]
		}, &tmp)
}

//========================================================================================

func do_decode(read read_fn, src *interface{}) *Object {
	b := []byte{0}
	read(src, b, 1)
	mask := id_pack_type(b[0])

	switch mask.group {
	case grp_ARRAY:
		return decode_array(read, src, b[0], mask)
	case grp_MAP:
		return decode_map(read, src, b[0], mask)
	case grp_STRING:
		return decode_string(read, src, b[0], mask)
	case grp_NLINT, grp_INT:
		return decode_integer(read, src, b[0], mask)
	case grp_PLINT, grp_UINT:
		return decode_unsigned(read, src, b[0], mask)
	case grp_EXT:
		return decode_ext(read, src, b[0], mask)
	case grp_BOOL:
		return decode_bool(mask)
	case grp_NIL:
		return decode_nil()
	case grp_BIN:
		panic("Bin is not implemented.")
	default:
		log.Panicf("Default reached. grp: %d, obj: %v", mask.group, mask)
	}
	return nil
}

//========================================================================================

func decode_array(read read_fn, src *interface{}, b byte, mask *mpack_mask) *Object {
	var (
		item Object
		size uint32
		word = []byte{0, 0, 0, 0}
	)

	if mask.fixed {
		size = uint32(b ^ mask.val)
	} else {
		switch mask.mtype {
		case m_ARRAY_16:
			read(src, word, 2)
			size = uint32(decode_uint16(word))
		case m_ARRAY_32:
			read(src, word, 4)
			size = decode_uint32(word)
		default:
			panic(errmsg)
		}
	}

	item.flags |= mFLAG_ENCODE
	item.Mtype = T_ARRAY
	item.Data = make([]Object, size)

	// Eprintf("\n\nIt is an array! -> 0x%0X => size %d\n\n", b, size)

	for i := range item.Data.([]Object) {
		item.Data.([]Object)[i] = *do_decode(read, src)
	}

	return &item
}

func decode_map(read read_fn, src *interface{}, b byte, mask *mpack_mask) *Object {
	var (
		item Object
		size uint32
		word = []byte{0, 0, 0, 0}
	)

	if mask.fixed {
		size = uint32(b ^ mask.val)
	} else {
		switch mask.mtype {
		case m_MAP_16:
			read(src, word, 2)
			size = uint32(decode_uint16(word))
		case m_MAP_32:
			read(src, word, 4)
			size = decode_uint32(word)
		default:
			panic(errmsg)
		}
	}

	item.Mtype = T_MAP
	item.flags = mFLAG_ENCODE
	item.Data = make([]Map_Entry, size)

	for i := range item.Data.([]Map_Entry) {
		item.Data.([]Map_Entry)[i].Key = *do_decode(read, src)
		item.Data.([]Map_Entry)[i].Value = *do_decode(read, src)
	}

	return &item
}

func decode_string(read read_fn, src *interface{}, b byte, mask *mpack_mask) *Object {
	var (
		item Object
		size uint32
		word = []byte{0, 0, 0, 0}
	)

	if mask.fixed {
		size = uint32(b ^ mask.val)
	} else {
		switch mask.mtype {
		case m_STR_8:
			read(src, word, 1)
			size = uint32(word[0])
		case m_STR_16:
			read(src, word, 2)
			size = uint32(decode_uint16(word))
		case m_STR_32:
			read(src, word, 4)
			size = decode_uint32(word)
		default:
			panic(errmsg)
		}
	}

	item.Mtype = T_STRING
	item.flags = mFLAG_ENCODE
	item.Data = make([]byte, size)

	read(src, item.Data.([]byte), uint(size))

	return &item
}

func decode_integer(read read_fn, src *interface{}, b byte, mask *mpack_mask) *Object {
	var (
		item  Object
		value int64
		word  = []byte{0, 0, 0, 0, 0, 0, 0, 0}
	)

	if mask.fixed {
		value = int64(b ^ mask.val)
		value = int64(uint64(value) | 0xFFFFFFFFFFFFFFE0)
	} else {
		switch mask.mtype {
		case m_INT_8:
			read(src, word, 1)
			value = int64(word[0])
			value = int64(uint64(value) | 0xFFFFFFFFFFFFFF00)
		case m_INT_16:
			read(src, word, 2)
			value = int64(decode_int16(word))
			value = int64(uint64(value) | 0xFFFFFFFFFFFF0000)
		case m_INT_32:
			read(src, word, 4)
			value = int64(decode_int32(word))
			value = int64(uint64(value) | 0xFFFFFFFF00000000)
		case m_INT_64:
			read(src, word, 8)
			value = decode_int64(word)
		default:
			panic(errmsg)
		}
	}

	item.flags = mFLAG_ENCODE
	item.Mtype = T_NUM
	item.Data = value

	return &item
}

func decode_unsigned(read read_fn, src *interface{}, b byte, mask *mpack_mask) *Object {
	var (
		item  Object
		value uint64
		word  = []byte{0, 0, 0, 0, 0, 0, 0, 0}
	)

	if mask.fixed {
		value = uint64(b ^ mask.val)
	} else {
		switch mask.mtype {
		case m_UINT_8:
			read(src, word, 1)
			value = uint64(word[0])
		case m_UINT_16:
			read(src, word, 2)
			value = uint64(decode_uint16(word))
		case m_UINT_32:
			read(src, word, 4)
			value = uint64(decode_uint32(word))
		case m_UINT_64:
			read(src, word, 8)
			value = decode_uint64(word)
		default:
			panic(errmsg)
		}
	}

	item.flags = mFLAG_ENCODE
	item.Mtype = T_NUM
	item.Data = int64(value)

	return &item
}

func decode_ext(read read_fn, src *interface{}, b byte, mask *mpack_mask) *Object {
	var (
		item  Object
		value uint32
		word  = []byte{0, 0, 0, 0}
		t     = []byte{0}
	)

	if mask.fixed {
		value = uint32(b ^ mask.val)
	} else {
		switch mask.mtype {
		case m_EXT_F1:
			read(src, t, 1)
			read(src, word, 1)
			value = uint32(word[0])
		case m_EXT_F2:
			read(src, t, 1)
			read(src, word, 2)
			value = uint32(decode_uint16(word))
		case m_EXT_F4:
			read(src, t, 1)
			read(src, word, 4)
			value = decode_uint32(word)
		default:
			panic(errmsg)
		}
	}

	item.flags = mFLAG_ENCODE
	item.Mtype = T_EXT
	item.Data = Ext{int8(t[0]), value}

	return &item
}

func decode_bool(mask *mpack_mask) *Object {
	var item Object
	item.Mtype = T_BOOL
	item.flags = mFLAG_ENCODE

	switch mask.mtype {
	case m_TRUE:
		item.Data = true
	case m_FALSE:
		item.Data = false
	default:
		panic(errmsg)
	}

	return &item
}

func decode_nil() *Object {
	var item Object
	item.Mtype = T_NIL
	item.flags = mFLAG_ENCODE
	item.Data = int16(0)

	return &item
}

//========================================================================================

func id_pack_type(b byte) *mpack_mask {
	var ret *mpack_mask

	/* For some reason range doesn't seem to process this array in the correct order,
	 * which is required for this to work. */
	for i := 0; i < len(m_masks); i++ {
		m := m_masks[i]

		if m.fixed {
			if (b >> m.shift) == (m.val >> m.shift) {
				ret = &m
				break
			}
		} else {
			if b == m.val {
				ret = &m
				break
			}
		}
	}

	if ret == nil {
		panic("Failed to id pack.")
	}

	return ret
}
