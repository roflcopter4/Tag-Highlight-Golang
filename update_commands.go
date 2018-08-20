package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"tag_highlight/api"
	"tag_highlight/mpack"
	"tag_highlight/scan"
	"tag_highlight/util"
)

var (
	update_mutex sync.Mutex
)

//========================================================================================

func (bdata *Bufdata) Update_Highlight() {
	update_mutex.Lock()
	defer update_mutex.Unlock()
	api.Echo("Updating highlight commands for bufnum %d", bdata.Num)
	timer := util.NewTimer()

	if !bdata.Ft.Restore_Cmds_Init {
		tmp := api.Nvim_get_var(0, []byte("tag_highlight#restored_groups"), mpack.E_MAP_STR_BYTELIST).(map[string][][]byte)
		restored_groups := tmp[bdata.Ft.Vim_Name]
		if restored_groups != nil {
			bdata.Ft.Restore_Cmds = get_restore_cmds(restored_groups)
			// api.Echo("%s", bdata.Ft.Restore_Cmds)
		}
		bdata.Ft.Restore_Cmds_Init = true
	}

	if bdata.Calls != nil {
		bdata.update_from_cache()
		return
	}

	scanner := bdata.Make_Scan_Struct()
	tags := scanner.Scan(logdir)

	if util.Logfiles["taglst"] != nil {
		for i, t := range tags {
			fmt.Fprintf(util.Logfiles["taglst"], "Tag %d: %c - %s\n", i, t.Kind, t.Str)
		}
		util.Logfiles["taglst"].Sync()
	}

	if tags != nil {
		api.Echo("Found %d total tags", len(tags))
		// bdata.update_commands(tags)
		bdata.update_commands(tags)
		api.Nvim_call_atomic(0, bdata.Calls)

		if bdata.Ft.Restore_Cmds != nil {
			util.Logfiles["cmds"].Write(bdata.Ft.Restore_Cmds)
			api.Nvim_command(0, bdata.Ft.Restore_Cmds)
		}
	}
	timer.EchoReport("update highlight")

	// util.Logfiles["cmds"].Sync()
}

func (bdata *Bufdata) Make_Scan_Struct() *scan.Bufdata {
	return &scan.Bufdata{
		Vimbuf:       bdata.Lines,
		RawTags:      bdata.Topdir.Tags,
		Ignored_Tags: bdata.Ft.Ignored_Tags,
		Equiv:        bdata.Ft.Equiv,
		Order:        bdata.Ft.Order,
		Filename:     []byte(bdata.Filename),
		Lang:         []byte(strings.ToLower(bdata.Ft.Ctags_Name)),
		Id:           int(bdata.Ft.Id),
		Is_C:         bdata.Topdir.Is_C,
	}
}

//========================================================================================

func get_restore_cmds(restored_groups [][]byte) []byte {
	allcmds := make([][]byte, 0, len(restored_groups))

	for _, group := range restored_groups {
		cmd := make([]byte, 0, 128)
		append_all(&cmd, []byte("syntax list "), group)

		output := api.Nvim_command_output(0, cmd, mpack.E_BYTES)

		if output == nil {
			continue
		}
		str := output.([]byte)
		// api.Echo("\nlen: %d, STRING: '%s'", len(str), str)

		i := bytes.Index(str, []byte("xxx"))
		if i == (-1) {
			continue
		}
		i += 4
		// api.Echo("i: %d, STRING: '%s'", i, str[i:])

		cmd = make([]byte, 0, 128)
		cmd = append(cmd, fmt.Sprintf("syntax clear %s | ", group)...)
		toks := [][]byte{{}}

		if !bytes.Equal(str[i:i+7], []byte("match /")) && !bytes.Equal(str[i:i+5], []byte("start")) {
			// i += 7
			append_all(&cmd, []byte("syntax keyword "), group, []byte(" "))

			// api.Echo("i: %d, STRING: '%s'", i, str[i:])
			n := bytes.IndexByte(str[i:], '\n')

			for ; n != (-1); n = bytes.IndexByte(str[i:], '\n') {
				n += i
				if n >= len(str) {
					panic("AAA")
				}
				// api.Echo("i: %d, STRING: '%s'", i, str[i:])
				// api.Echo("n: %d, FOUND:  '%s'", n, str[n:])
				toks = append(toks, str[i:n])

				for str[n] == ' ' || str[n] == '\t' || str[n] == '\n' || str[n] == '\r' {
					n++
				}
				i = n
				if bytes.Equal(str[n:n+9], []byte("links to ")) {
					n = bytes.IndexByte(str[i:], '\n')
					if n != (-1) && (n >= len(str) || len(str[n:]) == 0) {
						break
					}
				}
			}

			i += 9
			// toks = util.Unique_Str(toks)
			sort.Slice(toks, func(i, x int) bool { return bytes.Compare(toks[i], toks[x]) >= 0 })

			cmd = append(cmd, append(toks[0], ' ')...)

			for x := 1; x < len(toks); x++ {
				if !bytes.Equal(toks[x], toks[x-1]) {
					cmd = append(cmd, append(toks[x], ' ')...)
				}
			}

			link_name := str[i:]
			util.Assert(i > 0, "No link name")
			cmd = append(cmd, fmt.Sprintf("| hi! link %s %s", group, link_name)...)

			allcmds = append(allcmds, cmd)
		}
	}

	return bytes.Join(allcmds, []byte(" | "))
}

//========================================================================================

type cmd_info struct {
	group  []byte
	prefix []byte
	suffix []byte
	kind   byte
}

func (bdata *Bufdata) update_commands(tags []scan.Tag) {
	ngroups := len(bdata.Ft.Order)
	info := make([]cmd_info, ngroups)

	for i := 0; i < ngroups; i++ {
		ch := bdata.Ft.Order[i]
		tmp := api.Nvim_get_var_fmt(0, mpack.E_MAP_STR_BYTES, "%s#%s#%c",
			"tag_highlight", bdata.Ft.Vim_Name, ch).(map[string][]byte)

		info[i] = cmd_info{tmp["group"], tmp["prefix"], tmp["suffix"], ch}
	}

	// bdata.Calls = new(api.Atomic_list)
	bdata.Calls = &api.Atomic_list{Calls: make([]api.Atomic_call, 0, 2048)}
	bdata.Calls.Nvim_command([]byte("ownsyntax"))

	for i := 0; i < ngroups; i++ {
		var ctr int
		for ctr = 0; ctr < len(tags); ctr++ {
			if tags[ctr].Kind == info[i].kind {
				break
			}
		}

		if ctr != len(tags) {
			cmd := handle_kind(ctr, bdata.Ft, &info[i], tags)
			// util.Logfiles["cmds"].WriteString(cmd + "\n")
			bdata.Calls.Nvim_command(cmd)
		}
	}

	util.Logfiles["cmds"].Write([]byte("\n\n\n\n"))
}

func handle_kind(i int, ft *Ftdata, info *cmd_info, tags []scan.Tag) []byte {
	group_id := []byte(fmt.Sprintf("_tag_highlight_%s_%c_%s", ft.Vim_Name, info.kind, info.group))
	cmd := []byte(fmt.Sprintf("silent! syntax clear %s | ", group_id))

	if info.prefix != nil || info.suffix != nil {
		prefix := []byte("\\C\\<")
		suffix := []byte("\\>")

		if info.prefix != nil {
			prefix = info.prefix
		}
		if info.suffix != nil {
			suffix = info.suffix
		}

		cmd = append(cmd, fmt.Sprintf("syntax match %s /%s\\%%(%s", group_id, prefix, tags[i].Str)...)
		i++

		for ; i < len(tags) && tags[i].Kind == info.kind; i++ {
			cmd = append(cmd, []byte("\\|")...)
			cmd = append(cmd, tags[i].Str...)
		}

		cmd = append(cmd, fmt.Sprintf("\\)%s/ display | hi def link %s %s", suffix, group_id, info.group)...)
	} else {
		cmd = append(cmd, fmt.Sprintf(" syntax keyword %s %s ", group_id, tags[i].Str)...)
		i++

		for ; i < len(tags) && tags[i].Kind == info.kind; i++ {
			cmd = append(cmd, tags[i].Str...)
			cmd = append(cmd, ' ')
		}

		cmd = append(cmd, fmt.Sprintf("display | hi def link %s %s", group_id, info.group)...)
	}

	return cmd
}

//========================================================================================

func (bdata *Bufdata) update_from_cache() {
	api.Echo("Updating from cache")
	api.Nvim_call_atomic(0, bdata.Calls)
	if bdata.Ft.Restore_Cmds != nil {
		api.Nvim_command(0, bdata.Ft.Restore_Cmds)
		// api.Echo("%s", bdata.Ft.Restore_Cmds)
	}
}

func append_all(to *[]byte, a ...interface{}) {
	for _, src := range a {
		*to = append(*to, src.([]byte)...)
	}
}
