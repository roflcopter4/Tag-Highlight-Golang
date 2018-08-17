package neotags

/*
#cgo CFLAGS: -march=native -g -O3
#define DEBUG
#define HAVE_VASPRINTF
#define HAVE_REALLOCARRAY
#define LZMA_SUPPORT
#define _GNU_SOURCE
#include "neotags.h"

static struct tag *
index_taglist(struct taglist *list, const unsigned index)
{
	if (!list || !list->lst || index > list->qty)
		return NULL;

	return list->lst[index];
}

static void
destroy_taglist(struct taglist *tags)
{
	if (!tags)
		return;
	for (unsigned i = 0; i < tags->qty; ++i) {
		b_destroy(tags->lst[i]->b);
		free(tags->lst[i]);
	}
	free(tags->lst);
	free(tags);
}
*/
import "C"
import "unsafe"

type Bufdata struct {
	Equiv        [][2]byte
	Ignored_Tags []string
	Filename     string
	Order        string
	Ctags_Name   string
	Id           int
}
type Tag struct {
	Str  string
	Kind byte
}

// var free_me = make([]unsafe.Pointer, 0, 256)

//========================================================================================

func Everything(bdata *Bufdata, govimbuf string, tagfile []string) []Tag {
	var (
		cvimbuf  = s2bstr(govimbuf)
		cdata    = bdata.make_c_data()
		ctagfile = slice2blist(tagfile)
	)
	cvimbuf = C.strip_comments(cdata, cvimbuf)
	var (
		toks = C.tokenize(cvimbuf, cdata.id)
		tags = C.process_tags(cdata, ctagfile, toks)
		ret  = convert_taglist(tags)
	)

	C.b_list_destroy(ctagfile)
	C.b_list_destroy(toks)
	C.b_free(cvimbuf)
	C.destroy_taglist(tags)
	destroy_c_data(cdata)

	return ret
}

//========================================================================================

func (bdata *Bufdata) make_c_data() *C.struct_bufdata {
	ret := (*C.struct_bufdata)(C.malloc(C.sizeof_struct_bufdata))

	ret.equiv = equiv_to_blist(bdata.Equiv)
	ret.ignored_tags = slice2blist(bdata.Ignored_Tags)
	ret.filename = s2bstr(bdata.Filename)
	ret.order = s2bstr(bdata.Order)
	ret.ctags_name = s2bstr(bdata.Ctags_Name)
	ret.id = C.enum_filetype_id(bdata.Id)

	return ret
}

func destroy_c_data(data *C.struct_bufdata) {
	C.b_list_destroy(data.equiv)
	C.b_list_destroy(data.ignored_tags)
	C.b_free(data.filename)
	C.b_free(data.order)
	C.b_free(data.ctags_name)
	C.free(unsafe.Pointer(data))
}

func convert_taglist(clist *C.struct_taglist) []Tag {
	if clist == nil {
		return nil
	}
	len := int(clist.qty)
	ret := make([]Tag, len)

	for i := 0; i < len; i++ {
		tag := C.index_taglist(clist, C.uint(i))
		ret[i] = Tag{
			Str:  bstr2gostr(tag.b),
			Kind: byte(tag.kind),
		}
	}

	return ret
}

//========================================================================================

// func Malloc(size C.ulong) unsafe.Pointer {
//         ret := C.malloc(size)
//         free_me = append(free_me, ret)
//         return ret
// }

func equiv_to_blist(equiv [][2]byte) *C.b_list {
	list := C.b_list_create_alloc(C.uint(len(equiv)))
	for _, elem := range equiv {
		C.b_list_append(&list, b2bstr(elem[:]))
	}
	return list
}

func slice2blist(slc []string) *C.b_list {
	list := C.b_list_create_alloc(C.uint(len(slc)))
	for _, str := range slc {
		C.b_list_append(&list, s2bstr(str))
	}
	return list
}

func s2bstr(str string) *C.bstring {
	tmp := C.CString(str)
	return c2bstr(tmp, len(str))
}

func b2bstr(str []byte) *C.bstring {
	tmp := C.CString(string(str))
	return c2bstr(tmp, len(str))
}

func c2bstr(str *C.char, size int) *C.bstring {
	bstr := (*C.bstring)(C.malloc(C.sizeof_bstring))

	bstr.data = (*C.uchar)(unsafe.Pointer(str))
	bstr.slen = C.uint(size)
	bstr.mlen = C.uint(size)
	bstr.flags = C.BSTR_STANDARD

	return bstr
}

func bstr2gostr(bstr *C.bstring) string {
	if bstr == nil || bstr.data == nil {
		return ""
	}
	return C.GoStringN(
		(*C.char)(unsafe.Pointer(bstr.data)), C.int(bstr.slen))
}
