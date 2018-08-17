package mpack

import (
	"bytes"
	"fmt"
	"os"
	"sync"
)

var (
	mpack_print_mutex sync.Mutex
	recursion         int
	indent            int
	pendl             bool
)

//========================================================================================

func (obj *Object) Print(fp *os.File) {
	if !DEBUG || fp == nil {
		return
	}

	mpack_print_mutex.Lock()
	recursion = 0
	indent = 0
	pendl = true
	print_object(fp, obj)
	mpack_print_mutex.Unlock()
}

func print_object(fp *os.File, obj *Object) {
	recursion++
	switch obj.Mtype {
	case T_ARRAY:
		print_array(fp, obj)
	case T_MAP:
		print_map(fp, obj)
	case T_STRING:
		print_string(fp, obj, true)
	case T_EXT:
		print_ext(fp, obj)
	case T_NIL:
		print_nil(fp)
	case T_BOOL:
		print_bool(fp, obj)
	case T_NUM:
		print_number(fp, obj)
	default:
		panic(fmt.Sprintf("Invalid pack type '%s'.", obj.TypeRepr()))
	}
}

func pindent(fp *os.File) {
	if indent <= 0 {
		return
	}
	fp.Write(bytes.Repeat([]byte(" "), indent*4))
}

func end(fp *os.File, str []byte) {
	if pendl {
		str = append(str, byte('\n'))
	}

	fp.Write(str)
}

//========================================================================================

func print_array(fp *os.File, obj *Object) {
	pindent(fp)

	if len(obj.Data.([]Object)) == 0 {
		end(fp, []byte("[]"))
	} else {
		fp.Write([]byte("[\n"))
		indent++

		for _, elem := range obj.Data.([]Object) {
			print_object(fp, &elem)
		}

		indent--
		pindent(fp)
		end(fp, []byte("]"))
	}
}

func print_map(fp *os.File, obj *Object) {
	pindent(fp)
	fp.Write([]byte("{\n"))
	indent++

	for _, ent := range obj.Data.([]Map_Entry) {
		pindent(fp)
		pendl = false
		if ent.Key.Mtype == T_STRING {
			print_string(fp, &ent.Key, false)
		} else {
			print_object(fp, &ent.Key)
		}

		// fp.Seek(-1, os.SEEK_CUR)
		tmp := indent
		pendl = true

		switch ent.Value.Mtype {
		case T_ARRAY, T_MAP:
			fp.Write([]byte("  =>  (\n"))

			indent++
			print_object(fp, &ent.Value)
			indent--

			pindent(fp)
			fp.Write([]byte(")\n"))
		default:
			fp.Write([]byte("  =>  "))

			indent = 0
			print_object(fp, &ent.Value)
			indent = tmp
		}
	}

	indent--
	pindent(fp)
	end(fp, []byte("}"))
}

func print_string(fp *os.File, obj *Object, ind bool) {
	if ind {
		pindent(fp)
	}
	s := fmt.Sprintf("\"%s\"", obj.Data.([]byte))
	end(fp, []byte(s))
}

func print_ext(fp *os.File, obj *Object) {
	pindent(fp)
	s := fmt.Sprintf("EXT: (%d -> %d)",
		obj.Data.(Ext).Etype, obj.Data.(Ext).Num)
	end(fp, []byte(s))
}

func print_nil(fp *os.File) {
	pindent(fp)
	end(fp, []byte("NIL"))
}

func print_bool(fp *os.File, obj *Object) {
	pindent(fp)
	if obj.Data.(bool) {
		end(fp, []byte("true"))
	} else {
		end(fp, []byte("false"))
	}
}

func print_number(fp *os.File, obj *Object) {
	pindent(fp)
	s := fmt.Sprintf("%d", obj.Data.(int64))
	end(fp, []byte(s))
}
