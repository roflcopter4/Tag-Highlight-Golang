package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"tag_highlight/mpack"
	"time"
)

type bstr = []byte

type settings_t struct {
	Comp_type       uint16
	Comp_level      uint16
	Ctags_args      []string
	Enabled         bool
	Ignored_ftypes  []string
	Ignored_tags    map[string][]string
	Norecurse_dirs  []string
	Settings_file   string
	Use_compression bool
	Verbose         bool
}

var (
	Logfiles      = make(map[string]*os.File, 10)
	read_fd  int  = (-1)
	Sockfd   int  = (-1)
	DEBUG    bool = false
	HOME     string
	Settings settings_t
)

const ( // Compression Types
	COMP_GZIP = iota
	COMP_LZMA
	COMP_NONE
)

//========================================================================================

func main() {
	mpack.DEBUG = true
	DEBUG = true
	Logfiles["nvim"] = safe_fopen("nvim.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	Logfiles["main"] = safe_fopen("main.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer Logfiles["nvim"].Close()
	defer Logfiles["main"].Close()

	{
		var b bool
		HOME, b = os.LookupEnv("HOME")
		if !b {
			panic("Home directory not set.")
		}
	}

	// read_fd = safe_open(HOME+"/decode_raw.log", os.O_RDONLY, 0644)
	read_fd = 0
	// defer syscall.Close(read_fd)

	Sockfd = create_socket()
	Eprintf("Created socket fd %d!\n", Sockfd)

	Settings = settings_t{
		Comp_type:       get_compression_type(0),
		Comp_level:      uint16(Nvim_get_var(0, pkg("compression_level"), mpack.T_NUM).(int64)),
		Ctags_args:      Nvim_get_var(0, pkg("ctags_args"), mpack.G_STRLIST).([]string),
		Enabled:         Nvim_get_var(0, pkg("enabled"), mpack.T_BOOL).(bool),
		Ignored_ftypes:  Nvim_get_var(0, pkg("ignore"), mpack.G_STRLIST).([]string),
		Ignored_tags:    Nvim_get_var(0, pkg("ignored_tags"), mpack.G_MAP_STR_STRLIST).(map[string][]string),
		Norecurse_dirs:  Nvim_get_var(0, pkg("norecurse_dirs"), mpack.G_STRLIST).([]string),
		Settings_file:   Nvim_get_var(0, pkg("settings_file"), mpack.G_STRING).(string),
		Use_compression: Nvim_get_var(0, pkg("use_compression"), mpack.T_BOOL).(bool),
		Verbose:         Nvim_get_var(0, pkg("verbose"), mpack.T_BOOL).(bool),
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
			fsleep(3.0)
			echo("Retrying initial connection (attempt %d)", attempts)
		}

		initial_buf = Nvim_get_current_buf(0)
		if New_Buffer(0, initial_buf) != nil {
			go main_loop(initial_buf)
			/* Nvim_buf_attach(1, initial_buf)
			ret1 := Decode_Nvim_Stream(1, MES_ANY)
			ret2 := Decode_Nvim_Stream(1, MES_ANY)
			var event *mpack.Object

			if ret1.Index(0).Get_Expect(mpack.T_NUM).(int64) == MES_NOTIFICATION {
				event = ret1
			} else if ret2.Index(0).Get_Expect(mpack.T_NUM).(int64) == MES_NOTIFICATION {
				event = ret2
			} else {
				panic("Didn't recieve an update from neovim...")
			}

			handle_nvim_event(event) */
		}
	}

	for {
		time.Sleep(1000 * time.Minute)
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
		Nvim_buf_attach(sock, bufnum)
		ret1 := Decode_Nvim_Stream(sock, MES_ANY)
		ret2 := Decode_Nvim_Stream(sock, MES_ANY)
		var event *mpack.Object

		if ret1.Index(0).Get_Expect(mpack.T_NUM).(int64) == MES_NOTIFICATION {
			event = ret1
		} else if ret2.Index(0).Get_Expect(mpack.T_NUM).(int64) == MES_NOTIFICATION {
			event = ret2
		} else {
			panic("Didn't recieve an update from neovim...")
		}

		handle_nvim_event(event)
	}

	for {
		event := Decode_Nvim_Stream(sock, MES_NOTIFICATION)
		event.Print(Logfiles["main"])
		handle_nvim_event(event)
	}
}

//========================================================================================

func pkg(v string) string {
	// This little monument to laziness should pretty much always get inlined.
	return "tag_highlight#" + v
}

func create_socket() int {
	name := Nvim_call_function(1, "serverstart", mpack.G_STRING).(string)

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
	tmp := Nvim_get_var(0, pkg("compression_type"), mpack.G_STRING).(string)
	var ret uint16 = COMP_NONE

	switch tmp {
	case "gzip":
		ret = COMP_GZIP
	case "lzma":
		ret = COMP_LZMA
	case "none":
		ret = COMP_NONE
	default:
		echo("Warning: unrecognized compression type \"%s\", defaulting to no compression.", tmp)
	}
	echo("Compression type is '%s' -> %d", tmp, ret)

	return ret
}
