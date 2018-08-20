package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	sys "syscall"
	"tag_highlight/api"
	"tag_highlight/archive"
	"tag_highlight/util"
)

//========================================================================================

func (bdata *Bufdata) Run_Ctags(force int) bool {
	if bdata == nil || bdata.Topdir == nil {
		panic("Nil paramaters")
	}

	var headers []string = nil
	if bdata.Topdir.Is_C {
		headers = find_headers(bdata)
	}

	status := exec_ctags(bdata, headers, force)

	api.Echo("Status: %d", status)
	return (status == 0)
}

func (bdata *Bufdata) Get_Initial_Taglist() bool {
	timer := util.NewTimer()
	defer timer.EchoReport("Initial Taglist")
	bdata.Topdir.Tags = make([][]byte, 0, 128)
	var (
		do_ctags       = false
		e        error = nil
	)

	if have_seen_file(bdata.Filename) {
		api.Echo("Seen file before, running ctags in case there was just a momentary disconnect on write...")
		e = errors.New("")
	} else {
		_, e = os.Stat(bdata.Topdir.Gzfile)
	}

	if e == nil {
		api.Echo("Reading gzfile '%s'", bdata.Topdir.Gzfile)
		if bdata.Topdir.Read_Gzfile(int(Settings.Comp_type)) {
			if err := bdata.Topdir.Write_Tmpfile(); err != nil {
				util.Warn("Error writing tag file: %s\n", err)
			}
		} else {
			if !bdata.Initialized {
				return false
			}
			util.Warn("Could not read tag file; running ctags")
			do_ctags = true
		}
	} else {
		do_ctags = true
	}

	if do_ctags {
		if os.IsNotExist(e) {
			api.Echo("File '%s' not found, running ctags", bdata.Topdir.Gzfile)
		} else if e != nil && e.Error() != "" {
			util.Warn("Unexpected io error: %v", e)
		}

		if !bdata.Run_Ctags(0) {
			util.Warn("Ctags failed...")
		}
		if err := bdata.Topdir.Read_Tmpfile(); err != nil {
			util.Warn("Read error: %v", err)
		}
		if !bdata.Topdir.Write_Gzfile(int(Settings.Comp_type)) {
			api.Echo("Error writing gzfile")
			return false
		}
	}

	return true
}

func (bdata *Bufdata) Update_Taglist(force int) bool {
	timer := util.NewTimer()
	defer timer.EchoReport("Update Taglist")
	if force == 0 && bdata.Ctick == bdata.Last_Ctick {
		api.Echo("ctick unchanged, not updating")
		return false
	}

	bdata.Last_Ctick = bdata.Ctick
	if !bdata.Run_Ctags(force) {
		util.Warn("Ctags failed...")
	}

	if err := bdata.Topdir.Read_Tmpfile(); err != nil {
		api.Echo("Error reading temporary file '%s'", err)
		return false
	}

	if !bdata.Topdir.Write_Gzfile(int(Settings.Comp_type)) {
		api.Echo("Error writing gzfile")
		return false
	}

	bdata.Calls = nil

	return true
}

//========================================================================================

type tdata struct {
	wg         *sync.WaitGroup
	ret        *[]string
	searched   *[]string
	src_dirs   []string
	cur_header string
}

func find_headers(bdata *Bufdata) []string {
	includes := find_includes(bdata)
	if includes == nil {
		util.Warn("includes is nil...")
		return nil
	}
	src_dirs := find_src_dirs(bdata, includes)
	if src_dirs == nil {
		util.Warn("src_dirs is nil...")
		return nil
	}
	headers := find_header_paths(src_dirs, includes)
	if headers == nil {
		util.Warn("headers is nil...")
		return nil
	}
	if len(headers) == 0 {
		util.Warn("No headers found at all...")
		return nil
	}

	includes = util.Unique_Str(includes)
	var (
		hcopy    = make([]string, len(headers))
		searched = make([]string, 0, 32)
		data     = make([]tdata, len(headers))
		wg       sync.WaitGroup
	)
	copy(hcopy, headers)

	for i, file := range hcopy {
		tmp := make([]string, 0, 32)
		data[i] = tdata{
			wg:         &wg,
			ret:        &tmp,
			searched:   &searched,
			src_dirs:   src_dirs,
			cur_header: file,
		}
		wg.Add(1)
		go recurse_headers(&data[i], 1)
	}

	wg.Wait()

	for i := range data {
		if data[i].ret != nil {
			headers = append(headers, *data[i].ret...)
		}
	}

	return util.Unique_Str(headers)
}

func find_includes(bdata *Bufdata) []string {
	includes := make([]string, 32)
	dirname := filepath.Dir(bdata.Filename)

	for node := bdata.Lines.Head; node != nil; node = node.Next {
		file := analyze_line(node.Data.(string))
		if file != "" {
			includes = append(includes, file, dirname)
		}
	}

	if len(includes) == 0 {
		includes = nil
	}
	return includes
}

func find_src_dirs(bdata *Bufdata, includes []string) []string {
	src_dirs := make([]string, 0, 32)

	/***********************************
	 *      PARSE JSON FILE HERE       *
	 ***********************************/

	src_dirs = append(src_dirs, bdata.Topdir.Pathname)
	file_dir := filepath.Dir(bdata.Filename)
	if bdata.Topdir.Pathname != file_dir {
		src_dirs = append(src_dirs, file_dir)
	}

	if len(src_dirs) == 0 {
		src_dirs = nil
	}
	return src_dirs
}

//========================================================================================

const max_HEADER_SEARCH_LEVEL = 8

var searched_mutex sync.Mutex

func recurse_headers(data *tdata, level int) {
	if level > max_HEADER_SEARCH_LEVEL || data.cur_header == "" {
		return
	}
	searched_mutex.Lock()
	if level > 1 {
		for _, str := range *data.searched {
			if data.cur_header == str {
				searched_mutex.Unlock()
				return
			}
		}
	}
	*data.searched = append(*data.searched, data.cur_header)
	searched_mutex.Unlock()

	var (
		dirname  = filepath.Dir(data.cur_header)
		includes = make([]string, 0, 32)
		slurp    = util.Quick_Read(data.cur_header)
		lines    = bytes.Split(slurp, []byte("\n"))
	)

	for _, ln := range lines {
		str := string(ln)
		if file := analyze_line(str); file != "" {
			includes = append(includes, file, dirname)
		}
	}

	headers := find_header_paths(data.src_dirs, includes)
	// api.Echo("Appending %s", headers)
	if headers != nil {
		*data.ret = append(*data.ret, headers...)

		for _, file := range headers {
			tmp := tdata{
				wg:         nil,
				ret:        data.ret,
				searched:   data.searched,
				src_dirs:   data.src_dirs,
				cur_header: file,
			}
			recurse_headers(&tmp, level+1)
		}
	}
	if level == 1 && data.wg != nil {
		defer data.wg.Done()
	}
}

func analyze_line(line string) string {
	i, m := 0, len(line)
	ret := ""

	if m > 0 && line[i] == '#' {
		i++
		skip_space(&line, &i)
		if i < m && strings.HasPrefix(line[i:], "include") {
			i += 7
			skip_space(&line, &i)
			if i < m {
				ch := line[i]
				if ch == '"' || ch == '<' {
					i++
					end := strings.IndexByte(line[i:], ch)
					if end != (-1) {
						end += i
						ret = line[i:end]
					}
				}
			}
		}
	}

	return ret
}

//========================================================================================

func find_file_in_dir_recurse(dirpath, find string) string {
	files, e := ioutil.ReadDir(dirpath)
	if e != nil {
		panic(e)
	}

	var (
		ret      = ""
		namelist = make([]string, 0, len(files))
		dirlist  = make([]string, 0, len(files))
	)

	for _, cur := range files {
		if cur.IsDir() {
			dirlist = append(dirlist, cur.Name())
		} else {
			namelist = append(namelist, cur.Name())
		}
	}

	if i := sort.SearchStrings(namelist, find); i < len(namelist) {
		ret = filepath.Join(dirpath, namelist[i])
	} else {
		for _, dir := range dirlist {
			ret = find_file_in_dir_recurse(filepath.Join(dirpath, dir), find)
		}
	}

	return ret
}

func find_header_paths(src_dirs, includes []string) []string {
	headers := make([]string, 0, len(includes))

	for i := 0; i < len(includes); i += 2 {
		var (
			file = includes[i]
			path = includes[i+1]
			tmp  = filepath.Join(path, file)
		)
		if file == "" || path == "" {
			continue
		}

		if _, e := os.Stat(tmp); e == nil {
			headers = append(headers, tmp)
			includes[i] = ""
		}
		includes[i+1] = ""
	}

	for _, dir := range src_dirs {
		for _, file := range includes {
			if file != "" {
				tmp := filepath.Join(dir, file)
				if _, e := os.Stat(tmp); e == nil {
					headers = append(headers, tmp)
				}
			}
		}
	}

	if len(headers) == 0 {
		headers = nil
	}
	return headers
}

//========================================================================================
// Read and write the temporary file

var tmp_mutex sync.Mutex

func (topdir *TopDir) Read_Gzfile(comp_type int) bool {
	tmp_mutex.Lock()
	defer tmp_mutex.Unlock()
	topdir.Tags = archive.ReadFile(topdir.Gzfile, comp_type)
	return (topdir.Tags != nil)
}

func (topdir *TopDir) Read_Tmpfile() error {
	tmp_mutex.Lock()
	defer tmp_mutex.Unlock()

	if topdir.Tmpfd == (-1) {
		return errors.New("File not open")
	}
	var st sys.Stat_t

	if err := sys.Fstat(int(topdir.Tmpfd), &st); err != nil {
		util.Warn("Error stat'ing temporary file")
		return err
	}

	buf := make([]byte, st.Size)
	sys.Seek(int(topdir.Tmpfd), 0, os.SEEK_SET)
	rlen, err := sys.Read(int(topdir.Tmpfd), buf)

	util.Assert(int64(rlen) == st.Size && err == nil,
		fmt.Sprintf("Read error (%d of %d bytes read): %s", rlen, st.Size, err))
	topdir.Tags = bytes.Split(buf, []byte("\n"))
	return err
}

func (topdir *TopDir) Write_Gzfile(comp_type int) bool {
	tmp_mutex.Lock()
	defer tmp_mutex.Unlock()
	return archive.WriteFile(topdir.Gzfile, topdir.Tags, comp_type)
}

func (topdir *TopDir) Write_Tmpfile() error {
	tmp_mutex.Lock()
	defer tmp_mutex.Unlock()
	if topdir.Tmpfd == (-1) {
		return errors.New("File not open")
	}
	buf := bytes.Join(topdir.Tags, []byte("\n"))

	_, err := sys.Write(int(topdir.Tmpfd), buf)
	return err
}

//========================================================================================

func skip_space(str *string, i *int) {
	for *i < len(*str) && ((*str)[*i] == ' ' || (*str)[*i] == '\t') {
		*i++
	}
}

func exec_ctags(bdata *Bufdata, headers []string, force int) int {
	argv := make([]string, 0, len(headers)+32)
	argv = append(argv, "--", "ctags")
	argv = append(argv, Settings.Ctags_args...)
	argv = append(argv, "-f"+bdata.Topdir.Tmpfname)

	if (force != 2) && bdata.Topdir.Recurse && !bdata.Topdir.Is_C {
		argv = append(argv, "--languages="+bdata.Ft.Ctags_Name, "-R", bdata.Topdir.Pathname)
	} else {
		if bdata.Topdir.Is_C {
			argv = append(argv, "--languages=c,c++")
		} else {
			argv = append(argv, "--languages-force="+bdata.Ft.Ctags_Name)
		}

		argv = append(argv, bdata.Filename)

		if headers != nil {
			sort.Strings(headers)
			argv = append(argv, headers...)
		}
	}

	api.Echo("Executing '/usr/bin/env -- ctags' with args '%s'", strings.Join(argv[2:], ", "))

	var (
		pid  int
		ws   sys.WaitStatus
		err  error
		attr = get_procattr()
	)

	pid, err = sys.ForkExec("/usr/bin/env", argv, attr)
	if err != nil {
		panic(err)
	}
	if _, err = sys.Wait4(pid, &ws, 0, nil); err != nil {
		panic(err)
	}

	return ws.ExitStatus()
}

func get_procattr() *sys.ProcAttr {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &sys.ProcAttr{
		Dir:   dir,
		Env:   os.Environ(),
		Files: []uintptr{0, 1, 2},
	}
}
