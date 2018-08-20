package scan

/*
#cgo CFLAGS: -march=native -g -O3
#include <stdlib.h>

typedef char * string_t_;

struct strlist {
        unsigned   qty;
        unsigned   mlen;
        char     **lst;
        unsigned  *lengths;
};

static int
index_len(const struct strlist *list, const int index)
{
	return (int)(list->lengths[index]);
}

static char *
index_string(const struct strlist *list, const int index)
{
	return list->lst[index];
}

// #define DEREF_SIZE(OBJ_, IND_) ((int)((OBJ_)->lengths[IND_]))

extern size_t strip_comments(char **buffer, int64_t size);
extern struct strlist *tokenize(char *vimbuf, const int id);
*/
// import "C"

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"tag_highlight/api"
	"tag_highlight/lists"
	"tag_highlight/util"
	"time"
	// "unsafe"
)

const ( // Filetypes
	FT_NONE = iota
	FT_C
	FT_CPP
	FT_CSHARP
	FT_GO
	FT_JAVA
	FT_JAVASCRIPT
	FT_LISP
	FT_PERL
	FT_PHP
	FT_PYTHON
	FT_RUBY
	FT_RUST
	FT_SHELL
	FT_VIM
	FT_ZSH
)

type Bufdata struct {
	Vimbuf       *lists.Linked_List
	Ignored_Tags [][]byte
	RawTags      [][]byte
	Filename     []byte
	Lang         []byte
	Order        []byte
	Equiv        map[rune]rune
	Id           int
	Is_C         bool
}
type Tag struct {
	Str  []byte
	Kind byte
}
type TagList []Tag

var (
	scan_mutex sync.Mutex
	logdir     string
	mainlog    *os.File
	is_c       bool
)

//========================================================================================

func (bdata *Bufdata) Scan(ldir string) []Tag {
	scan_mutex.Lock()
	defer scan_mutex.Unlock()

	is_c = bdata.Is_C
	logdir = ldir
	mainlog = util.Safe_Fopen(filepath.Join(logdir, "search.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer mainlog.Close()

	// var (
	//         vimbuf = bdata.Vimbuf.Join("\n")
	//         cstr   = C.CString(vimbuf)
	//         slen   = C._strip_comments(&cstr, C.int64_t(len(vimbuf)))
	//         result = C.GoStringN(cstr, C.int(slen))
	// )

	// tv3 := time.Now()
	// vimbuf := bdata.Vimbuf.Join([]byte("\n"))
	// tv4 := time.Now()
	// api.Echo("Time used joining linked list: %f", util.Tdiff(&tv3, &tv4))
	//
	// tv3 = time.Now()
	// cstr := C.CString(vimbuf)
	// tv4 = time.Now()
	// api.Echo("Time used just making cstring: %f", util.Tdiff(&tv3, &tv4))
	//
	// tv3 = time.Now()
	//
	// slen := C.strip_comments(&cstr, C.int64_t(len(vimbuf)))
	// // strlist := C.tokenize(cstr, C.int(bdata.Id))
	// // toks := make([]string, int(strlist.qty))
	//
	// // for i := 0; i < int(strlist.qty); i++ {
	// //         toks[i] = C.GoStringN(C.index_string(strlist, C.int(i)), C.index_len(strlist, C.int(i)))
	// //         C.free(unsafe.Pointer(C.index_string(strlist, C.int(i))))
	// //
	// //         // siz := C.DEREF_SIZE(strlist, i)
	// //         // toks[i] = C.GoStringN(
	// //         //         *(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(strlist.lst)) + uintptr(i*int(C.sizeof_string_t_)))),
	// //         //         *(*C.int)(unsafe.Pointer(uintptr(unsafe.Pointer(strlist.lengths)) + uintptr(i*int(C.sizeof_unsigned)))))
	// //         // C.free(unsafe.Pointer(*(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(strlist.lst)) + uintptr(i*int(C.sizeof_string_t_))))))
	// // }
	// // C.free(unsafe.Pointer(strlist.lst))
	// // C.free(unsafe.Pointer(strlist.lengths))
	// // C.free(unsafe.Pointer(strlist))
	//
	// tv4 = time.Now()
	// api.Echo("Time used running C code: %f", util.Tdiff(&tv3, &tv4))
	//
	// tv3 = time.Now()
	// result := C.GoStringN(cstr, C.int(slen))
	// tv4 = time.Now()
	// api.Echo("Time used making go string: %f", util.Tdiff(&tv3, &tv4))
	//
	// C.free(unsafe.Pointer(cstr))
	// tv2 := time.Now()
	// api.Echo("Time used after C code returns: %f", util.Tdiff(&tv1, &tv2))

	timer1 := util.NewTimer()
	tv1 := time.Now()

	vimbuf := bdata.Vimbuf.Join([]byte("\n"))

	// result := util.TimeRoutine("stripping comments", func(a ...interface{}) interface{} {
	//         bdata := a[0].(*Bufdata)
	//         vimbuf := a[1].(string)
	//         return bdata.Strip_Comments(vimbuf)
	// }, bdata, vimbuf).(string)

	timer2 := util.NewTimer()
	result := bdata.Strip_Comments(vimbuf)
	timer2.EchoReport("stripping comments")

	api.Echo("Got back len %d", len(result))
	dump("stripped.log", result)

	timer2.Reset()
	toks := tokenize(result, bdata.Id)
	timer2.EchoReport("tokenizing")

	tags := bdata.scan_tags(toks)

	timer1.EchoReport("all scan operations")
	api.Echo("or -> %.10f", time.Since(tv1).Seconds())
	return tags
}

func dump(fname string, data []byte) {
	if file, e := os.Create(filepath.Join(logdir, fname)); e != nil {
		panic(e)
	} else {
		file.Write(data)
		file.Close()
	}
}

//========================================================================================

func (bdata *Bufdata) scan_tags(vimbuf [][]byte) []Tag {
	if bdata.RawTags == nil || vimbuf == nil || len(bdata.RawTags) == 0 || len(vimbuf) == 0 {
		return nil
	}

	var (
		tagcpy   = make([][]byte, len(bdata.RawTags))
		nthreads = runtime.NumCPU() * 3
		quot     = len(bdata.RawTags) / nthreads
		tags     = make([][]Tag, nthreads)
		wg       sync.WaitGroup
	)
	copy(tagcpy, bdata.RawTags)

	// tv1 := time.Now()
	timer := util.NewTimer()

	for i := 0; i < nthreads; i++ {
		var num int
		if i == (nthreads - 1) {
			num = len(tagcpy) - ((nthreads - 1) * quot)
		} else {
			num = quot
		}

		wg.Add(1)
		go do_search(&wg, &tags[i], bdata, vimbuf, tagcpy[i*quot:(i*quot)+num], num, i)
	}

	wg.Wait()
	// tv2 := time.Now()
	// api.Echo("Searching time: time: %f", util.Tdiff(&tv1, &tv2))
	timer.EchoReport("search")

	ret := []Tag{}
	for _, elem := range tags {
		ret = append(ret, elem...)
	}

	if len(ret) == 0 {
		return nil
	}

	timer.Reset()

	sort.Sort(sort.Interface((*TagList)(&ret)))
	api.Echo("Got %d tags", len(ret))
	ret = remove_dups(ret)
	api.Echo("Of which %d are unique", len(ret))

	timer.EchoReport("Sorting")

	return ret
}

func do_search(wg *sync.WaitGroup, tags *[]Tag, bdata *Bufdata, vimbuf, rawtags [][]byte, num, threadnum int) {
	defer wg.Done()
	if len(vimbuf) == 0 {
		return
	}
	*tags = make([]Tag, 0, (len(vimbuf)*2)/3)

	for i := 0; i < num; i++ {
		// if rawtags == nil || rawtags[i] == nil || rawtags[i][0] == '!' {
		if len(rawtags) == 0 || len(rawtags[i]) == 0 || rawtags[i][0] == '!' {
			continue
		}

		var (
			split      [][]byte = bytes.Split(rawtags[i], []byte("\t"))
			name       []byte   = split[0]
			match_file []byte   = split[1]
			match_lang []byte
			kind       byte
		)
		split = split[2:]

		for _, tok := range split {
			if len(tok) == 1 {
				kind = tok[0]
			} else if bytes.HasPrefix(tok, []byte("language:")) {
				match_lang = tok[9:]
			}
		}

		if kind == 0 || len(match_lang) == 0 {
			continue
		}

		if in_order(bdata.Equiv, bdata.Order, &kind) &&
			is_correct_lang(bdata.Lang, &match_lang) &&
			!skip_tag(bdata.Ignored_Tags, name) &&
			(bytes.Equal(bdata.Filename, match_file) ||
				inbuf(vimbuf, name)) {
			*tags = append(*tags, Tag{name, kind})
		}

		// rej := func(s string) {
		//         flog("rejecting tag (reason: %s): s: %s, k: %c", s, name, kind)
		// }

		// if !in_order(bdata.Equiv, bdata.Order, &kind) {
		//         rej("not in order")
		// } else if !is_correct_lang(bdata.Lang, &match_lang) {
		//         rej("wrong language (" + match_lang + " vs " + bdata.Lang)
		// } else if skip_tag(bdata.Ignored_Tags, name) {
		//         rej("in skip list")
		// } else if bdata.Filename != match_file && !inbuf(vimbuf, name) { //!( [> bdata.Filename == match_file || <] x < len(vimbuf)-1) {
		//         rej("not in buffer")
		// } else {
		//         flog("Accepting tag: s: %s, k: %c\n", name, kind)
		//         *tags = append(*tags, Tag{name, kind})
		// }
	}

	if len(*tags) > 2 {
		*tags = remove_dups(*tags)
	}

	// return ret
}

func flog(f string, a ...interface{}) {
	fmt.Fprintf(mainlog, f, a...)
}

//========================================================================================

func in_order(equiv map[rune]rune, order []byte, kind *byte) bool {
	if equiv != nil {
		tmp := rune(*kind)
		for key, item := range equiv {
			if tmp == key {
				*kind = byte(item)
			}
		}
	}

	return (bytes.IndexByte(order, *kind) != (-1))
}

func is_correct_lang(lang []byte, match *[]byte) bool {
	if (*match)[len(*match)-1] == '\r' {
		*match = (*match)[:len(*match)-1]
	}

	*match = bytes.ToLower(*match)

	if bytes.Equal(lang, *match) {
		return true
	}

	return (is_c && (bytes.Equal(*match, []byte("c")) || bytes.Equal(*match, []byte("c++"))))
}

func skip_tag(skip [][]byte, find []byte) bool {
	if skip != nil && len(skip) != 0 {
		for _, s := range skip {
			if bytes.Equal(s, find) {
				return true
			}
		}
	}
	return false
}

func inbuf(vimbuf [][]byte, name []byte) bool {
	x := sort.Search(len(vimbuf), func(i int) bool { return bytes.Compare(vimbuf[i], name) >= 0 })
	return x < len(vimbuf) && bytes.Equal(vimbuf[x], name)
}

//========================================================================================

func remove_dups(tags []Tag) []Tag {
	sort.Sort(sort.Interface((*TagList)(&tags)))
	ret := make([]Tag, 1, len(tags))
	ret[0] = tags[0]

	for i := 1; i < len(tags); i++ {
		if tags[i].Kind != tags[i-1].Kind || !bytes.Equal(tags[i].Str, tags[i-1].Str) {
			ret = append(ret, tags[i])
		}
	}

	return ret
}

func (tags *TagList) Len() int {
	return len(*tags)
}

func (tags *TagList) Less(a, b int) bool {
	var ret int
	tA := (*tags)[a]
	tB := (*tags)[b]

	if tA.Kind == tB.Kind {
		if len(tA.Str) == len(tB.Str) {
			ret = bytes.Compare(tA.Str, tB.Str)
		} else {
			ret = len(tA.Str) - len(tB.Str)
		}
	} else {
		ret = int(tA.Kind) - int(tB.Kind)
	}

	return ret < 0
}

func (tags *TagList) Swap(a, b int) {
	tmp := (*tags)[a]
	(*tags)[a] = (*tags)[b]
	(*tags)[b] = tmp
}
