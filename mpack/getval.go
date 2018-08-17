package mpack

import (
	"log"
)

//========================================================================================

func errm(obj *Object, expected string) {
	log.Printf("WARNING: Expected type '%s' but pack is type '%s'\n",
		expected, obj.TypeRepr())
}

func (obj *Object) Get_Int64() int64 {
	var ret int64 = 0

	switch obj.Mtype {
	case T_NUM:
		ret = obj.Data.(int64)
	case T_BOOL:
		val := obj.Data.(bool)
		if val {
			ret = 1
		} else {
			ret = 0
		}
	default:
		errm(obj, "int64")
	}

	return ret
}

func (obj *Object) Get_Int() int {
	return int(obj.Get_Int64())
}

func (obj *Object) Get_Bool() bool {
	var ret bool = false

	switch obj.Mtype {
	case T_BOOL:
		ret = obj.Data.(bool)
	case T_NUM:
		val := obj.Data.(int64)
		ret = (val == 1)
	default:
		errm(obj, "bool")
	}

	return ret
}

func (obj *Object) Get_String() string {
	if obj.Mtype != T_STRING {
		errm(obj, "string")
		return ""
	}
	return obj.Data.(string)
}

func (obj *Object) Get_StrList() []string {
	if obj.Mtype != T_ARRAY {
		errm(obj, "array")
		return nil
	}
	lst := make([]string, 0, 32)

	for _, elem := range obj.Data.([]Object) {
		if elem.Mtype == T_STRING {
			lst = append(lst, string(elem.Data.([]byte)))
		}
	}
	return lst
}

func (obj *Object) Get_IntList() []int {
	if obj.Mtype != T_ARRAY {
		errm(obj, "array")
		return nil
	}
	lst := make([]int, 0, 32)

	for _, elem := range obj.Data.([]Object) {
		lst = append(lst, elem.Get_Int())
	}
	return lst
}

func (obj *Object) Get_Map_Str_Str() map[string]string {
	if obj.Mtype != T_MAP {
		errm(obj, "map")
		return nil
	}
	return mpack_map_to_str_str(obj)
}

func (obj *Object) Get_Map_Str_StrList() map[string][]string {
	if obj.Mtype != T_MAP {
		errm(obj, "map")
		return nil
	}
	return mpack_map_to_str_strlist(obj)
}
