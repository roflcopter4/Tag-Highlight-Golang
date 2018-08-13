package mpack

import (
	"fmt"
)

const ( // Mpack object types
	T_UNINITIALIZED = iota
	T_BOOL
	T_NIL
	T_NUM
	T_EXT
	T_STRING
	T_ARRAY
	T_MAP
)

const ( // Mpack flags
	mFLAG_ENCODE = 0x01
	mFLAG_PACKED = 0x02
)
const ( // Mpack encoding masks
	mMASK_NIL       = uint8(0xC0)
	mMASK_TRUE      = uint8(0xC3)
	mMASK_FALSE     = uint8(0xC2)
	mMASK_ARRAY_F   = uint8(0x90)
	mMASK_ARRAY_16  = uint8(0xDC)
	mMASK_ARRAY_32  = uint8(0xDD)
	mMASK_MAP_F     = uint8(0x80)
	mMASK_MAP_16    = uint8(0xDE)
	mMASK_MAP_32    = uint8(0xDF)
	mMASK_STR_F     = uint8(0xA0)
	mMASK_STR_8     = uint8(0xD9)
	mMASK_STR_16    = uint8(0xDA)
	mMASK_STR_32    = uint8(0xDB)
	mMASK_BIN_8     = uint8(0xC4)
	mMASK_BIN_16    = uint8(0xC5)
	mMASK_BIN_32    = uint8(0xC6)
	mMASK_EXT_8     = uint8(0xD4)
	mMASK_EXT_16    = uint8(0xD5)
	mMASK_EXT_32    = uint8(0xD6)
	mMASK_INT_8     = uint8(0xD0)
	mMASK_INT_16    = uint8(0xD1)
	mMASK_INT_32    = uint8(0xD2)
	mMASK_INT_64    = uint8(0xD3)
	mMASK_UINT_8    = uint8(0xCC)
	mMASK_UINT_16   = uint8(0xCD)
	mMASK_UINT_32   = uint8(0xCE)
	mMASK_UINT_64   = uint8(0xCF)
	mMASK_POS_INT_F = uint8(0x00)
	mMASK_NEG_INT_F = uint8(0xE0)

	mARRAY_F_MAX = 15
)

var (
	DEBUG = true
)

var Type_Repr = [8]string{
	"mpack.T_UNINITIALIZED",
	"mpack.T_BOOL",
	"mpack.T_NIL",
	"mpack.T_NUM",
	"mpack.T_EXT",
	"mpack.T_STRING",
	"mpack.T_ARRAY",
	"mpack.T_MAP",
}

type Object struct {
	Data   interface{}
	Mtype  uint8
	flags  uint8
	packed []byte
}

type Map_Entry struct {
	Key, Value Object
}

type Ext struct {
	Etype int8
	Num   uint32
}

type Atomic_Args struct {
	Fmt  [][]byte
	Args []interface{}
}

//========================================================================================

func (obj *Object) Index(index int) *Object {
	if obj.Mtype != T_ARRAY {
		panic("Cannot index non-array")
	}

	return &(obj.Data.([]Object)[index])
}

func (obj *Object) MapEnt(index int) *Map_Entry {
	if obj.Mtype != T_MAP {
		panic("Cannot get map entry of non-map.")
	}

	return &(obj.Data.([]Map_Entry)[index])
}

func (obj *Object) GetPack() []byte {
	if (obj.flags & mFLAG_PACKED) != 0 {
		return obj.packed
	}
	return nil
}

//========================================================================================

const (
	mOWN_ARGS = iota
	mOTHER_ARGS
	mATOMIC_LIST
)

func two_thirds(val uint) uint {
	return ((val * 2) / 3)
}

func Encode_fmt(size_hint uint, format string, args ...interface{}) *Object {
	// eprintf("Fmt is '%s'\n", format)
	var (
		arr_size      uint    = 8192 + (6 * size_hint)
		sub_lengths   []uint  = make([]uint, arr_size)
		len_stack     []*uint = make([]*uint, two_thirds(arr_size))
		cur_len       *uint   = &sub_lengths[0]
		len_ctr       uint    = 0
		len_stack_ctr uint    = 0
	)

	for _, ch := range format {
		switch ch {
		case 'b', 'B', 'l', 'L', 'd', 'D', 's', 'S', 'c', 'C', 'n', 'N':
			*cur_len++

		case '[', '{':
			*cur_len++
			len_stack_ctr++
			len_ctr++
			len_stack[len_stack_ctr] = cur_len
			cur_len = &sub_lengths[len_ctr]

		case ']', '}':
			cur_len = len_stack[len_stack_ctr]

		case ':', '.', ' ', ',', '!', '@', '*':
		default:
			s := fmt.Sprintf("Illegal character '%c' in format.", ch)
			panic(s)
		}
	}

	if sub_lengths[0] == 0 {
		return nil
	}

	pack := Make_New(sub_lengths[0], true)

	var (
		ref             *[]interface{} = nil
		next_type       int            = mOWN_ARGS
		obj_stack       []*Object      = make([]*Object, two_thirds(arr_size))
		cur_obj         *Object        = pack.Index(0)
		map_stack       []uint         = make([]uint, two_thirds(arr_size))
		sub_ctrlist     []uint         = make([]uint, two_thirds(arr_size))
		cur_ctr         *uint          = &sub_ctrlist[0]
		obj_stack_ctr   uint           = 0
		map_stack_ctr   uint           = 0
		sub_ctrlist_ctr uint           = 0
		args_ctr        uint           = 0
		ref_ctr         uint           = 0
	)

	len_ctr = 0
	len_stack_ctr = 0
	len_stack[0] = cur_ctr
	obj_stack[0] = pack
	*cur_ctr = 1

	next_arg := func(atomic bool) interface{} {
		var ret interface{}

		switch next_type {
		case mOWN_ARGS:
			ret = args[args_ctr]
			args_ctr++
		case mOTHER_ARGS:
			ret = (*ref)[ref_ctr]
			ref_ctr++
		case mATOMIC_LIST:
			if !atomic {
				panic("Arg specified as atomic in illegal context.")
			}
			panic("Atomic not implemented yet!")
		default:
			panic("No type specified.")
		}

		return ret
	}

	inc_counters := func() {
		obj_stack_ctr++
		obj_stack[obj_stack_ctr] = cur_obj

		len_stack_ctr++
		len_stack[len_stack_ctr] = cur_ctr

		sub_ctrlist_ctr++
		cur_ctr = &sub_ctrlist[sub_ctrlist_ctr]
	}

	for _, ch := range format {
		switch ch {
		case 'b', 'B':
			var arg bool = next_arg(true).(bool)
			pack.Encode_Boolean(&cur_obj, arg)

		case 'd', 'D':
			var arg int = next_arg(true).(int)
			pack.Encode_Integer(&cur_obj, int64(arg))

		case 'l', 'L':
			var arg int64 = next_arg(true).(int64)
			pack.Encode_Integer(&cur_obj, arg)

		case 's', 'S':
			var arg string = next_arg(true).(string)
			pack.Encode_String(&cur_obj, []byte(arg))

		case 'c', 'C':
			var arg []byte = next_arg(true).([]byte)
			pack.Encode_String(&cur_obj, arg)

		case 'n', 'N':
			pack.Encode_Nil(&cur_obj)

		case '[':
			len_ctr++
			map_stack_ctr++
			map_stack[map_stack_ctr] = 0
			pack.Encode_Array(&cur_obj, sub_lengths[len_ctr])
			inc_counters()

		case '{':
			len_ctr++
			map_stack_ctr++
			map_stack[map_stack_ctr] = 1
			pack.Encode_Map(&cur_obj, sub_lengths[len_ctr]/2)
			inc_counters()

		case ']', '}':
			obj_stack_ctr--
			map_stack_ctr--
			len_stack_ctr--
			cur_ctr = len_stack[len_stack_ctr]

		case '!':
			ref = next_arg(false).(*[]interface{})
			next_type = mOTHER_ARGS
			continue

		case '@':
			panic("Atomic not implemented.")

		case '*':
			panic("Atomic not implemented.")

		case ':', '.', ' ', ',':
			continue

		default:
			panic("Not reachable.")
		}

		if map_stack[map_stack_ctr] != 0 {
			// if x := uint(cap(obj_stack[obj_stack_ctr].Data.(*Map).entries)); x > (*cur_ctr / 2) {
			if x := uint(cap(obj_stack[obj_stack_ctr].Data.([]Map_Entry))); x > (*cur_ctr / 2) {
				if (*cur_ctr & 1) == 0 {
					cur_obj = &(*obj_stack[obj_stack_ctr]).MapEnt(int(*cur_ctr / 2)).Key
				} else {
					cur_obj = &(*obj_stack[obj_stack_ctr]).MapEnt(int(*cur_ctr / 2)).Value
				}
			} else {
				// eprintf("current cap is smaller than element number (%d vs %d)\n", x, (*cur_ctr / 2))
			}
		} else {
			if x := uint(cap((*obj_stack[obj_stack_ctr]).Data.([]Object))); x > *cur_ctr {
				cur_obj = (*obj_stack[obj_stack_ctr]).Index(int(*cur_ctr))
			} else {
				// eprintf("current cap is smaller than element number (%d vs %d)\n", x, (*cur_ctr / 2))
			}
		}

		// mpack_dumb_dump(pack)

		*cur_ctr++
	}

	return pack
}

//========================================================================================

const ( // Additional possible paramaters for get_expect()
	G_B_STRLIST = iota + 256
	G_STRLIST
	G_STRPTRLIST
	G_STRING
	G_INTLIST
	G_MAP_STR_STR
	G_MAP_STR_STRLIST
	G_MAP_RUNE_RUNE
)

var expect_repr_strings = [8]string{
	"G_B_STRLIST",
	"G_STRLIST",
	"G_STRPTRLIST",
	"G_STRING",
	"G_INTLIST",
	"G_MAP_STR_STR",
	"G_MAP_STR_STRLIST",
	"G_MAP_RUNE_RUNE",
}

func (obj *Object) Get_Expect(expect int) interface{} {
	eprintf("Got type type %s (expected %s)\n",
		expect_repr(int(obj.Mtype)), expect_repr(expect))

	if obj.Mtype == T_NIL {
		eprintf("Expect: got nil value.\n")
		return nil
	}
	if obj.Mtype != uint8(expect) {
		switch obj.Mtype {
		case T_EXT:
			switch expect {
			case T_NUM:
				return int64(obj.Data.(Ext).Num)
			}
		case T_NUM:
			switch expect {
			case T_BOOL:
				val := obj.Data.(int64)
				if val == 0 {
					return false
				} else if val == 1 {
					return true
				}
			}
		case T_STRING:
			switch expect {
			case G_STRING:
				return string(obj.Data.([]byte))
			}
		case T_ARRAY:
			switch expect {
			case G_B_STRLIST:
				lst := make([][]byte, 0, 32)
				for _, elem := range obj.Data.([]Object) {
					if elem.Mtype == T_STRING {
						lst = append(lst, elem.Data.([]byte))
					}
				}
				/* if len(lst) == 0 {
					panic("No strings found in array.")
				} */
				return lst
			case G_STRLIST:
				lst := make([]string, 0, 32)
				for _, elem := range obj.Data.([]Object) {
					if elem.Mtype == T_STRING {
						lst = append(lst, string(elem.Data.([]byte)))
					}
				}
				/* if len(lst) == 0 {
					panic("No strings found in array.")
				} */
				return lst
			case G_STRPTRLIST:
				lst := make([]*string, 0, 32)
				for _, elem := range obj.Data.([]Object) {
					if elem.Mtype == T_STRING {
						tmp := string(elem.Data.([]byte))
						lst = append(lst, &tmp)
					}
				}
				/* if len(lst) == 0 {
					panic("No strings found in array.")
				} */
				return lst
			case G_INTLIST:
				lst := make([]int, 0, 32)
				for _, elem := range obj.Data.([]Object) {
					lst = append(lst, int(elem.Get_Expect(T_NUM).(int64)))
				}
				if len(lst) == 0 {
					panic("No integers found in array.")
				}
				return lst
			}
		case T_MAP:
			switch expect {
			case G_MAP_STR_STR:
				return mpack_map_to_str_str(obj)
			case G_MAP_STR_STRLIST:
				return mpack_map_to_str_strlist(obj)
			case G_MAP_RUNE_RUNE:
				return mpack_map_to_rune_rune(obj)
			}
		}

		eprintf("WARNING: Got unexpected value of type %s (expected %s)\n",
			expect_repr(int(obj.Mtype)), expect_repr(expect))
		return nil
	}

	switch expect {
	case T_ARRAY:
		return obj.Data.([]Object)
	case T_MAP:
		return obj.Data.([]Map_Entry)
	case T_STRING:
		return obj.Data.([]byte)
	case T_EXT:
		return obj.Data.(Ext)
	case T_NUM:
		return obj.Data.(int64)
	case T_BOOL:
		return obj.Data.(bool)
	default:
		panic_fmt("Invalid type given to expect (%s).", expect_repr(expect))
		panic("")
	}
}

func expect_repr(expect int) string {
	if expect < 256 {
		return Type_Repr[expect]
	}
	return expect_repr_strings[expect-256]
}

func mmap_ent_str_conv(obj *Object) string {
	if obj.Mtype == T_STRING {
		return string(obj.Data.([]byte))
	}

	eprintf("Object is not a string -> %s\n", expect_repr(int(obj.Mtype)))
	return fmt.Sprintf("%v", obj.Data)
}

func mpack_map_to_str_str(obj *Object) map[string]string {
	tmp := obj.Data.([]Map_Entry)
	var ret = make(map[string]string, len(tmp))

	for _, ent := range tmp {
		key := mmap_ent_str_conv(&ent.Key)
		val := mmap_ent_str_conv(&ent.Value)
		ret[key] = val
	}

	return ret
}

func mpack_map_to_str_strlist(obj *Object) map[string][]string {
	tmp := obj.Data.([]Map_Entry)
	var ret = make(map[string][]string, len(tmp))

	for _, ent := range tmp {
		key := mmap_ent_str_conv(&ent.Key)
		val := ent.Value.Get_Expect(G_STRLIST).([]string)
		ret[key] = val
	}

	return ret
}

func mpack_map_to_rune_rune(obj *Object) map[rune]rune {
	tmp := obj.Data.([]Map_Entry)
	var ret = make(map[rune]rune, len(tmp))

	for _, ent := range tmp {
		key := rune(ent.Key.Data.([]byte)[0])
		val := rune(ent.Value.Data.([]byte)[0])
		ret[key] = val
	}

	return ret
}
