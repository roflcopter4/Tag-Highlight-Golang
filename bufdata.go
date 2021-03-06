package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	sys "syscall"
	"tag_highlight/api"
	"tag_highlight/archive"
	"tag_highlight/lists"
	"tag_highlight/mpack"
	"tag_highlight/util"
)

type Ftdata struct {
	Equiv             map[rune]rune
	Ignored_Tags      [][]byte
	Order             []byte
	Restore_Cmds      []byte
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
	Tags     [][]byte
}

type Bufdata struct {
	Ctick       uint32
	Last_Ctick  uint32
	Num         uint16
	Initialized bool
	Filename    string
	Lines       *lists.Linked_List
	Calls       *api.Atomic_list
	Ft          *Ftdata
	Topdir      *TopDir
}

const init_bufs int = 4096
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

var (
	TopDir_List  []*TopDir
	ftdata_mutex sync.Mutex
	seen_files   = []string{}
)
var buffers struct {
	lst         [init_bufs]*Bufdata
	bad_bufs    []uint16
	mkr         int
	bad_buf_mkr int
}
var Ftdata_List = [16]Ftdata{
	{nil, nil, nil, nil, "NONE", "NONE", FT_NONE, false, false},
	{nil, nil, nil, nil, "c", "c", FT_C, false, false},
	{nil, nil, nil, nil, "cpp", "c++", FT_CPP, false, false},
	{nil, nil, nil, nil, "cs", "csharp", FT_CSHARP, false, false},
	{nil, nil, nil, nil, "go", "go", FT_GO, false, false},
	{nil, nil, nil, nil, "java", "java", FT_JAVA, false, false},
	{nil, nil, nil, nil, "javascript", "javascript", FT_JAVASCRIPT, false, false},
	{nil, nil, nil, nil, "lisp", "lisp", FT_LISP, false, false},
	{nil, nil, nil, nil, "perl", "perl", FT_PERL, false, false},
	{nil, nil, nil, nil, "php", "php", FT_PHP, false, false},
	{nil, nil, nil, nil, "python", "python", FT_PYTHON, false, false},
	{nil, nil, nil, nil, "ruby", "ruby", FT_RUBY, false, false},
	{nil, nil, nil, nil, "rust", "rust", FT_RUST, false, false},
	{nil, nil, nil, nil, "sh", "sh", FT_SHELL, false, false},
	{nil, nil, nil, nil, "vim", "vim", FT_VIM, false, false},
	{nil, nil, nil, nil, "zsh", "zsh", FT_ZSH, false, false},
}

//========================================================================================

func New_Buffer(fd, bufnum int) *Bufdata {
	for _, num := range buffers.bad_bufs {
		if uint16(bufnum) == num {
			util.Eprintf("Buf is bad\n")
			return nil
		}
	}

	ft_str := api.Nvim_buf_get_option(fd, bufnum, []byte("ft"), mpack.E_STRING).(string)
	ft := id_filetype(ft_str)
	if ft == nil {
		api.Echo("Failed to identify filetype '%s'.", ft)
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
		panic(fmt.Sprintf("Couldn't locate buffer %d.", bufnum))
	}

	return bdata
}

func Remove_Buffer(bufnum int) {
	index, bdata := find_buffer_index(bufnum)
	log_seen_file(bdata.Filename)

	if bdata.Topdir != nil {
		topdir := bdata.Topdir
		topdir.refs--
		if topdir.refs == 0 {
			sys.Close(int(topdir.Tmpfd))
			sys.Unlink(topdir.Tmpfname)

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
		Filename:    api.Nvim_buf_get_name(fd, bufnum),
		Ft:          ft,
		Num:         uint16(bufnum),
		Ctick:       0,
		Last_Ctick:  0,
		Initialized: false,
		Calls:       nil,
		Lines:       lists.New_List(),
		Topdir:      nil,
	}

	bdata.Lines.Append("")
	bdata.Topdir = init_topdir(fd, &bdata)

	return &bdata
}

func init_topdir(fd int, bdata *Bufdata) *TopDir {
	var (
		dirname = check_project_directories(filepath.Dir(bdata.Filename))
		recurse = check_norecurse_directories(dirname)
		is_c    = bdata.Ft.Id == FT_C || bdata.Ft.Id == FT_CPP
		base    string
	)

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

	api.Echo("Initializing new topdir \"%s\", ft %s", dirname, bdata.Ft.Vim_Name)

	tmp_fname := api.Nvim_call_function(fd, []byte("tempname"), mpack.E_STRING).(string)
	tmp := TopDir{
		Gzfile:   HOME + "/.vim_tags_go/",
		Id:       bdata.Ft.Id,
		Is_C:     is_c,
		Pathname: dirname,
		Recurse:  recurse,
		Tags:     nil,
		Tmpfd:    int16(util.Safe_Open(tmp_fname, sys.O_CREAT|sys.O_RDWR|sys.O_DSYNC, 0600)),
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
	case archive.COMP_GZIP:
		tmp.Gzfile += "." + bdata.Ft.Vim_Name + ".tags.gz"
	case archive.COMP_LZMA:
		tmp.Gzfile += "." + bdata.Ft.Vim_Name + ".tags.xz"
	case archive.COMP_NONE:
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
	ft.Order = api.Nvim_get_var_fmt(fd, mpack.E_BYTES, "tag_highlight#%s#order", ft.Vim_Name).([]byte)
	ft.Ignored_Tags = Settings.Ignored_tags[ft.Vim_Name]

	tmp := api.Nvim_get_var_fmt(fd, mpack.E_MAP_RUNE_RUNE, "tag_highlight#%s#equivalent", ft.Vim_Name)
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
		s := scanner.Text()
		api.Echo("Found text '%s'", s)
		if strings.HasPrefix(dirname, s) {
			api.Echo("Found prefix...")
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

	api.Echo("Using %d -> %s", x, candidates[x])
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

func have_seen_file(fname string) bool {
	for _, f := range seen_files {
		if fname == f {
			return true
		}
	}
	return false
}

func log_seen_file(fname string) {
	if !have_seen_file(fname) {
		seen_files = append(seen_files, fname)
	}
}
