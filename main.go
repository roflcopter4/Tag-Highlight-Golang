package main

import (
	"fmt"
	"os"
	// "os/signal"
	"runtime"
	// "runtime/pprof"
	"syscall"
	"tag_highlight/api"
	"tag_highlight/archive"
	"tag_highlight/mpack"
	"tag_highlight/util"
)

type bstr = []byte

type settings_t struct {
	Comp_type       uint16
	Comp_level      uint16
	Ignored_tags    map[string][][]byte
	Ctags_args      []string
	Ignored_ftypes  []string
	Norecurse_dirs  []string
	Settings_file   string
	Enabled         bool
	Use_compression bool
	Verbose         bool
}

var (
	read_fd int = (-1)
	// Sockfd   int  = (-1)
	DEBUG    bool
	HOME     string
	src_dir  string
	logdir   string
	Settings settings_t
)

//========================================================================================

func main() {
	timer := util.NewTimer()
	DEBUG = false
	mpack.DEBUG = false
	util.Logfiles = make(map[string]*os.File, 10)
	util.SetEcho(api.Echo)

	// signal.

	{
		var b bool
		HOME, b = os.LookupEnv("HOME")
		if !b {
			panic("Home directory not set.")
		}
		src_dir = HOME + "/go/src/tag_highlight"
	}
	logdir = src_dir + "/.logs"

	if e := syscall.Mkdir(logdir, 0755); e != nil && e != syscall.EEXIST {
		panic(e)
	}
	// Logfiles["nvim"] = util.safe_fopen(logdir+"/nvim.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// Logfiles["main"] = util.safe_fopen(logdir+"/main.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// Logfiles["cmds"] = util.safe_fopen(logdir+"/cms.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// defer Logfiles["nvim"].Close()
	// defer Logfiles["main"].Close()
	// defer Logfiles["cmds"].Close()
	// util.Logfiles["taglst"] = util.Safe_Fopen(logdir+"/tag_list.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// defer util.Logfiles["taglst"].Close()

	// read_fd = safe_open(HOME+"/decode_raw.log", os.O_RDONLY, 0644)
	read_fd = 0
	// defer syscall.Close(read_fd)

	// prof_f := util.Safe_Fopen(logdir+"/prof.prof", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	// err := pprof.StartCPUProfile(prof_f)
	// util.Assert(err == nil, fmt.Sprintf("couldn't start profiling... -> %v", err))
	// defer pprof.StopCPUProfile()
	// defer prof_f.Close()

	api.Sockfd = create_socket()
	util.Eprintf("Created socket fd %d!\n", api.Sockfd)

	Settings = settings_t{
		Comp_type:       get_compression_type(0),
		Comp_level:      uint16(api.Nvim_get_var(0, pkg("compression_level"), mpack.T_NUM).(int64)),
		Ctags_args:      api.Nvim_get_var(0, pkg("ctags_args"), mpack.E_STRLIST).([]string),
		Enabled:         api.Nvim_get_var(0, pkg("enabled"), mpack.T_BOOL).(bool),
		Ignored_ftypes:  api.Nvim_get_var(0, pkg("ignore"), mpack.E_STRLIST).([]string),
		Ignored_tags:    api.Nvim_get_var(0, pkg("ignored_tags"), mpack.E_MAP_STR_BYTELIST).(map[string][][]byte),
		Norecurse_dirs:  api.Nvim_get_var(0, pkg("norecurse_dirs"), mpack.E_STRLIST).([]string),
		Settings_file:   api.Nvim_get_var(0, pkg("settings_file"), mpack.E_STRING).(string),
		Use_compression: api.Nvim_get_var(0, pkg("use_compression"), mpack.T_BOOL).(bool),
		Verbose:         api.Nvim_get_var(0, pkg("verbose"), mpack.T_BOOL).(bool),
	}
	if !Settings.Enabled {
		os.Exit(0)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	var initial_buf int = (-1)

	for attempts := 0; buffers.mkr == 0; attempts++ {
		if attempts > 0 {
			if len(buffers.bad_bufs) > 0 {
				buffers.bad_bufs = []uint16{}
			}
			util.Fsleep(3.0)
			api.Echo("Retrying initial connection (attempt %d)", attempts)
		}

		initial_buf = api.Nvim_get_current_buf(0)
		if New_Buffer(0, initial_buf) != nil {
			bdata := Find_Buffer(initial_buf)
			api.Nvim_buf_attach(1, initial_buf)

			bdata.get_initial_lines()
			bdata.Get_Initial_Taglist()
			bdata.Update_Highlight()

			// var tv2 syscall.Timespec
			// syscall.C

			// tv2 := time.Now()
			// api.Echo("Initial startup time: %f", (float64(tv1.))/float64(time.Second))
			// api.Echo("Initial startup time: %s", time.Since(tv1).String())
			// syscall.Gettimeofday
			// api.Echo("Initial startup time: %.10f", util.Tdiff(&tv1, &tv2))
			// util.Timer("startup", &tv1, &tv2)
			timer.EchoReport("initialization")
		}
	}

	for {
		event := api.Decode_Nvim_Stream(1, api.MES_NOTIFICATION)
		if event != nil {
			event.Print(util.Logfiles["main"])
			handle_nvim_event(event)
		}
	}
}

func mpack_raw_str(bte []byte) string {
	if len(bte) == 0 {
		return ""
	}
	var s string
	for _, c := range bte {
		s += fmt.Sprintf("%02X ", c)
	}

	return s[:len(s)-1]
}

//========================================================================================

func main_loop(bufnum int) {
	sock := create_socket()

	{
		api.Nvim_buf_attach(sock, bufnum)
		ret1 := api.Decode_Nvim_Stream(sock, api.MES_ANY)
		ret2 := api.Decode_Nvim_Stream(sock, api.MES_ANY)
		var event *mpack.Object

		if ret1.Index(0).Expect(mpack.T_NUM).(int64) == api.MES_NOTIFICATION {
			event = ret1
		} else if ret2.Index(0).Expect(mpack.T_NUM).(int64) == api.MES_NOTIFICATION {
			event = ret2
		} else {
			panic("Didn't recieve an update from neovim...")
		}

		handle_nvim_event(event)
	}

	for {
		event := api.Decode_Nvim_Stream(sock, api.MES_NOTIFICATION)
		event.Print(util.Logfiles["main"])
		handle_nvim_event(event)
	}
}

//========================================================================================

func pkg(varname string) []byte {
	// This little monument to laziness should pretty much always get inlined.
	return []byte("tag_highlight#" + varname)
}

func create_socket() int {
	name := api.Nvim_call_function(1, []byte("serverstart"), mpack.E_STRING).(string)

	var addr syscall.SockaddrUnix
	addr.Name = name

	fd, e := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if e != nil {
		panic(e)
	}
	if e = syscall.Connect(fd, &addr); e != nil {
		panic(e)
	}

	return fd
}

func get_compression_type(fd int) uint16 {
	tmp := api.Nvim_get_var(0, pkg("compression_type"), mpack.E_STRING).(string)
	var ret uint16 = archive.COMP_NONE

	switch tmp {
	case "gzip":
		ret = archive.COMP_GZIP
	case "lzma":
		ret = archive.COMP_LZMA
	case "none":
		ret = archive.COMP_NONE
	default:
		api.Echo("Warning: unrecognized compression type \"%s\", defaulting to no compression.", tmp)
	}
	api.Echo("Compression type is '%s' -> %d", tmp, ret)

	return ret
}

func (bdata *Bufdata) get_initial_lines() {
	list := api.Nvim_buf_get_lines(0, int(bdata.Num), 0, (-1))
	if bdata.Lines.Qty == 1 {
		bdata.Lines.Delete_Node(bdata.Lines.Head)
	}
	bdata.Lines.Insert_Slice_After(bdata.Lines.Head, sl2i(list)...)
	bdata.Initialized = true
}

func testroutine(i int) int {
	util.Fsleep(float64(1))
	return i * i
}
