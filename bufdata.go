package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	ll "tag_highlight/linked_list"
	"tag_highlight/mpack"
)

type Ftdata struct {
	Equiv             map[rune]rune
	Ignored_Tags      []string
	Restore_Cmds      string
	Order             string
	Vim_Name          string
	Ctags_Name        string
	Id                uint16
	Initialized       bool
	Restore_Cmds_Init bool
}

type TopDir struct {
	Tmpfd    int16
	Id       uint16
	index    uint16
	refs     uint16
	Recurse  bool
	Is_C     bool
	Gzfile   string
	Pathname string
	Tmpfname string
	Tags     []bstr
}

type Bufdata struct {
	Ctick       uint32
	Last_Ctick  uint32
	Num         uint16
	Initialized bool
	Filename    string
	Cmd_Cache   []bstr
	Lines       *ll.Linked_List
	Ft          *Ftdata
	Topdir      *TopDir
}

const init_bufs int = 4096
const ( // File types
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

var (
	TopDir_List  []*TopDir
	ftdata_mutex sync.Mutex
)
var buffers struct {
	lst         [init_bufs]*Bufdata
	bad_bufs    []uint16
	mkr         int
	bad_buf_mkr int
}
var Ftdata_List = [16]Ftdata{
	{nil, nil, "", "", "NONE", "NONE", FT_NONE, false, false},
	{nil, nil, "", "", "c", "c", FT_C, false, false},
	{nil, nil, "", "", "cpp", "c++", FT_CPP, false, false},
	{nil, nil, "", "", "cs", "csharp", FT_CSHARP, false, false},
	{nil, nil, "", "", "go", "go", FT_GO, false, false},
	{nil, nil, "", "", "java", "java", FT_JAVA, false, false},
	{nil, nil, "", "", "javascript", "javascript", FT_JAVASCRIPT, false, false},
	{nil, nil, "", "", "lisp", "lisp", FT_LISP, false, false},
	{nil, nil, "", "", "perl", "perl", FT_PERL, false, false},
	{nil, nil, "", "", "php", "php", FT_PHP, false, false},
	{nil, nil, "", "", "python", "python", FT_PYTHON, false, false},
	{nil, nil, "", "", "ruby", "ruby", FT_RUBY, false, false},
	{nil, nil, "", "", "rust", "rust", FT_RUST, false, false},
	{nil, nil, "", "", "sh", "sh", FT_SHELL, false, false},
	{nil, nil, "", "", "vim", "vim", FT_VIM, false, false},
	{nil, nil, "", "", "zsh", "zsh", FT_ZSH, false, false},
}

//========================================================================================

func New_Buffer(fd, bufnum int) *Bufdata {
	for _, num := range buffers.bad_bufs {
		if uint16(bufnum) == num {
			Eprintf("Buf is bad\n")
			return nil
		}
	}

	ft_str := Nvim_buf_get_option(fd, bufnum, "ft", mpack.G_STRING).(string)
	ft := id_filetype(ft_str)
	if ft == nil {
		echo("Failed to identify filetype '%s'.", ft)
		add_bad_buf(bufnum)
		return nil
	}
	for _, s := range Settings.Ignored_ftypes {
		if ft_str == s {
			add_bad_buf(bufnum)
			return nil
		}
	}

	bdata := get_bufdata(fd, bufnum, ft)
	if bdata.Ft.Id != FT_NONE && !bdata.Ft.Initialized {
		init_filetype(fd, ft)
	}
	buffers.lst[buffers.mkr] = bdata
	buffers.mkr++

	return bdata
}

func Find_Buffer(bufnum int) *Bufdata {
	for i := 0; i < len(buffers.lst); i++ {
		tmp := buffers.lst[i]
		if tmp != nil && tmp.Num == uint16(bufnum) {
			return tmp
		}
	}
	return nil
}

func Null_Find_Buffer(bufnum int, bdata *Bufdata) *Bufdata {
	if bdata == nil {
		bdata = Find_Buffer(bufnum)
	}
	if bdata == nil || is_bad_buf(bufnum) {
		panic_fmt("Couldn't locate buffer %d.")
	}

	return bdata
}

func Remove_Buffer(bufnum int) {
	index, bdata := find_buffer_index(bufnum)
	if bdata.Topdir != nil {
		topdir := bdata.Topdir
		topdir.refs--
		if topdir.refs == 0 {
			syscall.Close(int(topdir.Tmpfd))
			syscall.Unlink(topdir.Tmpfname)

			var i int = (-1)
			for x, t := range TopDir_List {
				if t == topdir {
					i = x
					break
				}
			}
			if i == (-1) {
				panic("Couldn't locate topdir in global list.")
			}

			TopDir_List[i] = nil
			TopDir_List = append(TopDir_List[:i], TopDir_List[i+1:]...)
		}
	}

	buffers.lst[index] = nil
}

//========================================================================================

func get_bufdata(fd, bufnum int, ft *Ftdata) *Bufdata {
	bdata := Bufdata{
		Filename:    Nvim_buf_get_name(fd, bufnum),
		Ft:          ft,
		Num:         uint16(bufnum),
		Ctick:       0,
		Last_Ctick:  0,
		Initialized: false,
		Cmd_Cache:   nil,
		Lines:       ll.Make_New(),
		Topdir:      nil,
	}

	bdata.Lines.Append("")
	bdata.Topdir = init_topdir(fd, &bdata)

	return &bdata
}

func init_topdir(fd int, bdata *Bufdata) *TopDir {
	dirname := filepath.Dir(bdata.Filename)
	dirname = check_project_directories(dirname)
	recurse := check_norecurse_directories(dirname)
	is_c := bdata.Ft.Id == FT_C || bdata.Ft.Id == FT_CPP

	var base string
	if !recurse || is_c {
		base = bdata.Filename
	} else {
		base = dirname
	}

	for _, tdir := range TopDir_List {
		if tdir != nil && tdir.Id == bdata.Ft.Id && tdir.Pathname == base {
			tdir.refs++
			return tdir
		}
	}

	echo("Initializing new topdir \"%s\", ft %s", dirname, bdata.Ft.Vim_Name)

	tmp_fname := Nvim_call_function(fd, "tempname", mpack.G_STRING).(string)
	tmp := TopDir{
		Gzfile:   HOME + ".vim_tags_go/",
		Id:       bdata.Ft.Id,
		Is_C:     is_c,
		Pathname: dirname,
		Recurse:  recurse,
		Tags:     nil,
		Tmpfd:    int16(safe_open(tmp_fname, os.O_CREATE|os.O_RDWR|os.O_SYNC, 0600)),
		Tmpfname: tmp_fname,
		index:    uint16(len(TopDir_List)),
		refs:     1,
	}

	for _, ch := range base {
		if ch == '/' || ch == ':' || ch == '\\' {
			tmp.Gzfile += "__"
		} else {
			tmp.Gzfile += string(ch)
		}
	}

	switch Settings.Comp_type {
	case COMP_GZIP:
		tmp.Gzfile += "." + bdata.Ft.Vim_Name + ".tags.gz"
	case COMP_LZMA:
		tmp.Gzfile += "." + bdata.Ft.Vim_Name + ".tags.xz"
	case COMP_NONE:
		tmp.Gzfile += "." + bdata.Ft.Vim_Name + ".tags"
	}

	TopDir_List = append(TopDir_List, &tmp)

	return &tmp
}

func init_filetype(fd int, ft *Ftdata) {
	if ft.Initialized {
		return
	}
	ftdata_mutex.Lock()
	defer ftdata_mutex.Unlock()

	ft.Initialized = true
	ft.Order = Nvim_get_var_fmt(fd, mpack.G_STRING, "tag_highlight#%s#order", ft.Vim_Name).(string)
	ft.Ignored_Tags = Settings.Ignored_tags[ft.Vim_Name]

	tmp := Nvim_get_var_fmt(fd, mpack.G_MAP_RUNE_RUNE, "tag_highlight#%s#equivalent", ft.Vim_Name)
	switch tmp.(type) {
	case nil:
		ft.Equiv = nil
	case map[rune]rune:
		ft.Equiv = tmp.(map[rune]rune)
	default:
		panic("Got garbage return value.")
	}
}

//========================================================================================

func check_project_directories(dirname string) string {
	candidates := make([]string, 0, 32)
	fp, e := os.Open(Settings.Settings_file)
	if e != nil {
		return dirname
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		if s := scanner.Text(); strings.Contains(s, dirname) {
			candidates = append(candidates, s)
		}
	}

	if len(candidates) == 0 {
		return dirname
	}

	var x int = 0
	for i, _ := range candidates {
		if len(candidates[i]) > len(candidates[x]) {
			x = i
		}
	}

	return candidates[x]
}

func check_norecurse_directories(dirname string) bool {
	if Settings.Norecurse_dirs != nil {
		for _, s := range Settings.Norecurse_dirs {
			if dirname == s {
				return false
			}
		}
	}

	return true
}

//========================================================================================

func id_filetype(ft string) *Ftdata {
	for i, m := range Ftdata_List {
		if ft == m.Vim_Name {
			return &Ftdata_List[i]
		}
	}
	return nil
}

func add_bad_buf(bufnum int) {
	buffers.bad_bufs = append(buffers.bad_bufs, uint16(bufnum))
}

func is_bad_buf(bufnum int) bool {
	for _, buf := range buffers.bad_bufs {
		if buf == uint16(bufnum) {
			return true
		}
	}
	return false
}

func find_buffer_index(bufnum int) (int, *Bufdata) {
	for i := 0; i < len(buffers.lst); i++ {
		tmp := buffers.lst[i]
		if tmp != nil && tmp.Num == uint16(bufnum) {
			return i, tmp
		}
	}
	return (-1), nil
}
