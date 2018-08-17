package main

import (
	"fmt"
	"strings"
	"sync"
	"tag_highlight/mpack"
	"tag_highlight/neotags"
)

var (
	update_mutex sync.Mutex
)

//========================================================================================

func (bdata *Bufdata) Update_Highlight() {
	update_mutex.Lock()
	defer update_mutex.Unlock()
	echo("Updating highlight commands for bufnum %d", bdata.Num)

	if !bdata.Ft.Restore_Cmds_Init {
		tmp := Nvim_get_var(0, "tag_highlight#restored_groups", mpack.E_MAP_STR_STRLIST).(map[string][]string)
		restored_groups := tmp[bdata.Ft.Vim_Name]
		if restored_groups != nil {
			bdata.Ft.Restore_Cmds = get_restore_cmds(restored_groups)
			// echo("%s", bdata.Ft.Restore_Cmds)
		}
		bdata.Ft.Restore_Cmds_Init = true
	}

	if bdata.Calls != nil {
		bdata.update_from_cache()
		return
	}

	var buf string
	for node := bdata.Lines.Head; node != nil; node = node.Next {
		buf += node.Data.(string) + "\n"
	}

	tags := neotags.Everything(bdata.Make_Neotags_Struct(), buf, bdata.Topdir.Tags)

	if tags != nil {
		echo("Found %d total tags", len(tags))
		bdata.update_commands(tags)
		Nvim_call_atomic(0, bdata.Calls)

		if bdata.Ft.Restore_Cmds != "" {
			Logfiles["cmds"].WriteString(bdata.Ft.Restore_Cmds)
			Nvim_command(0, bdata.Ft.Restore_Cmds)
		}
	}

	// max := 0
	// for _, t := range tags {
	//         if len(t.Str) > max {
	//                 max = len(t.Str)
	//         }
	// }
	// for _, t := range tags {
	//         echo("Tag: %-*s Kind: %c", max+1, t.Str, t.Kind)
	// }

	Logfiles["cmds"].Sync()
}

func (bdata *Bufdata) Make_Neotags_Struct() *neotags.Bufdata {
	var (
		equiv = make([][2]byte, len(bdata.Ft.Equiv))
		i     = 0
	)
	for key, val := range bdata.Ft.Equiv {
		equiv[i][0] = byte(key)
		equiv[i][1] = byte(val)
		i++
	}

	return &neotags.Bufdata{
		Equiv:        equiv,
		Ignored_Tags: bdata.Ft.Ignored_Tags,
		Filename:     bdata.Filename,
		Order:        bdata.Ft.Order,
		Ctags_Name:   bdata.Ft.Ctags_Name,
		Id:           int(bdata.Ft.Id),
	}
}

//========================================================================================

func get_restore_cmds(restored_groups []string) string {
	allcmds := make([]string, 0, len(restored_groups))

	for _, group := range restored_groups {
		cmd := "syntax list " + group
		output := Nvim_command_output(0, cmd, mpack.E_STRING)

		if output == nil {
			continue
		}
		str := output.(string)
		// echo("\nlen: %d, STRING: '%s'", len(str), str)

		i := strings.Index(str, "xxx")
		if i == (-1) {
			continue
		}
		i += 4
		// echo("i: %d, STRING: '%s'", i, str[i:])

		cmd = "syntax clear " + group + " | "
		toks := []string{}

		if str[i:i+7] != "match /" && str[i:i+5] != "start" {
			// i += 7
			cmd += "syntax keyword " + group + " "

			// echo("i: %d, STRING: '%s'", i, str[i:])
			n := strings.IndexByte(str[i:], '\n')

			for ; n != (-1); n = strings.IndexByte(str[i:], '\n') {
				n += i
				if n >= len(str) {
					panic("AAA")
				}
				// echo("i: %d, STRING: '%s'", i, str[i:])
				// echo("n: %d, FOUND:  '%s'", n, str[n:])
				toks = append(toks, str[i:n])

				for str[n] == ' ' || str[n] == '\t' || str[n] == '\n' || str[n] == '\r' {
					n++
				}
				i = n
				if str[n:n+9] == "links to " {
					n = strings.IndexByte(str[i:], '\n')
					if n != (-1) && (n >= len(str) || len(str[n:]) == 0) {
						break
					}
				}
			}

			i += 9
			toks = unique_str(toks)
			for _, cur := range toks {
				cmd += cur + " "
			}

			link_name := str[i:]
			assert(i > 0, "No link name")
			cmd += fmt.Sprintf("| hi! link %s %s", group, link_name)

			allcmds = append(allcmds, cmd)
		}
	}

	return strings.Join(allcmds, " | ")
}

//========================================================================================

type cmd_info struct {
	group  string
	prefix string
	suffix string
	kind   byte
}

func (bdata *Bufdata) update_commands(tags []neotags.Tag) {
	ngroups := len(bdata.Ft.Order)
	info := make([]cmd_info, ngroups)

	for i := 0; i < ngroups; i++ {
		ch := bdata.Ft.Order[i]
		tmp := Nvim_get_var_fmt(0, mpack.E_MAP_STR_STR, "%s#%s#%c",
			"tag_highlight", bdata.Ft.Vim_Name, ch).(map[string]string)

		info[i] = cmd_info{tmp["group"], tmp["prefix"], tmp["suffix"], ch}
	}

	bdata.Calls = new(atomic_list)
	bdata.Calls.nvim_command("ownsyntax")

	for i := 0; i < ngroups; i++ {
		var ctr int
		for ctr = 0; ctr < len(tags); ctr++ {
			if tags[ctr].Kind == info[i].kind {
				break
			}
		}

		if ctr != len(tags) {
			cmd := handle_kind(ctr, bdata.Ft, &info[i], tags)
			Logfiles["cmds"].WriteString(cmd + "\n")
			bdata.Calls.nvim_command(cmd)
		}
	}

	Logfiles["cmds"].Write([]byte("\n\n\n\n"))
}

func handle_kind(i int, ft *Ftdata, info *cmd_info, tags []neotags.Tag) string {
	group_id := fmt.Sprintf("_tag_highlight_%s_%c_%s", ft.Vim_Name, info.kind, info.group)
	cmd := fmt.Sprintf("silent! syntax clear %s | ", group_id)

	if info.prefix != "" || info.suffix != "" {
		prefix := "\\C\\<"
		suffix := "\\>"

		if info.prefix != "" {
			prefix = info.prefix
		}
		if info.suffix != "" {
			suffix = info.suffix
		}

		cmd += fmt.Sprintf("syntax match %s /%s\\%%(%s", group_id, prefix, tags[i].Str)
		i++

		for ; i < len(tags) && tags[i].Kind == info.kind; i++ {
			cmd += "\\|" + tags[i].Str
		}

		cmd += fmt.Sprintf("\\)%s/ display | hi def link %s %s", suffix, group_id, info.group)
	} else {
		cmd += fmt.Sprintf(" syntax keyword %s %s ", group_id, tags[i].Str)
		i++

		for ; i < len(tags) && tags[i].Kind == info.kind; i++ {
			cmd += tags[i].Str + " "
		}

		cmd += fmt.Sprintf("display | hi def link %s %s", group_id, info.group)
	}

	return cmd
}

//========================================================================================

func (list *atomic_list) nvim_command(cmd string) {
	call := atomic_call{}
	call.args = make([]interface{}, 2)
	call.args[0] = "nvim_command"
	call.args[1] = cmd
	call.fmt = "s[s]"

	list.calls = append(list.calls, call)
}

//========================================================================================

func (bdata *Bufdata) update_from_cache() {
	echo("Updating from cache")
	Nvim_call_atomic(0, bdata.Calls)
	if bdata.Ft.Restore_Cmds != "" {
		Nvim_command(0, bdata.Ft.Restore_Cmds)
		// echo("%s", bdata.Ft.Restore_Cmds)
	}
}
