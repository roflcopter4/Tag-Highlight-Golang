package main

import (
	"fmt"
	"os"
	"sync"
	ll "tag_highlight/linked_list"
	"tag_highlight/mpack"
)

const ( // Event types
	event_BUF_LINES = iota
	event_BUF_CHANGED_TICK
	event_BUF_DETACH
	event_VIM_UPDATE
)
const write_buf_updates = true

type event_id struct {
	name string
	id   int
}

var (
	event_list = [4]event_id{
		{"nvim_buf_lines_event", event_BUF_LINES},
		{"nvim_buf_changedtick_event", event_BUF_CHANGED_TICK},
		{"nvim_buf_detach_event", event_BUF_DETACH},
		{"vim_event_update", event_VIM_UPDATE},
	}
	event_mutex sync.Mutex
)

//========================================================================================

func handle_nvim_event(event *mpack.Object) {
	event_mutex.Lock()
	defer event_mutex.Unlock()

	etype := id_event(event)
	info := event.Index(2)

	if etype.id == event_VIM_UPDATE {
		panic("Not implemented yet.")
	} else {
		bufnum := int(info.Index(0).Get_Expect(mpack.T_NUM).(int64))
		bdata := Find_Buffer(bufnum)
		if bdata == nil {
			panic("Update called on an uninitialized buffer.")
		}

		switch etype.id {
		case event_BUF_LINES:
			handle_line_event(bdata, info)
		case event_BUF_CHANGED_TICK:
			bdata.Ctick = uint32(info.Index(1).Get_Expect(mpack.T_NUM).(int64))
		case event_BUF_DETACH:
			echo("Detaching from buffer %d", bufnum)
			Remove_Buffer(bufnum)
		}
	}
}

//========================================================================================

func handle_line_event(bdata *Bufdata, data *mpack.Object) {
	if data.Index(5).Get_Expect(mpack.T_BOOL).(bool) {
		panic("Somehow the boolean argument is true!")
	}
	if write_buf_updates {
		write_buf(bdata.Lines)
	}

	bdata.Ctick = uint32(data.Index(1).Get_Expect(mpack.T_NUM).(int64))
	var (
		first     = int(data.Index(2).Get_Expect(mpack.T_NUM).(int64))
		last      = int(data.Index(3).Get_Expect(mpack.T_NUM).(int64))
		repl_list = data.Index(4).Get_Expect(mpack.G_STRLIST).([]string)
		diff      = last - first
		iters     = max_int(diff, len(repl_list))
		empty     = false
	)

	if len(repl_list) > 0 {
		if last == (-1) {
			/* Initial lines update. */
			if bdata.Lines.Qty == 1 {
				bdata.Lines.Delete_Node(bdata.Lines.Head)
				assert(bdata.Lines.Verify_Size(), "Verify failed.")
			}
			bdata.Lines.Insert_Slice_After(bdata.Lines.Head, 0, (-1), to_interface(repl_list)...)
		} else if bdata.Lines.Qty <= 1 && first == 0 && len(repl_list) == 1 && len(repl_list[0]) == 0 {
			/* Useless update, one empty string in an empty buffer.
			 * Just ignore it. */
			echo("Empty update, ignoring.")
			empty = true
		} else if first == 0 && last == 0 {
			/* Inserting above the first line in the file. */
			bdata.Lines.Insert_Slice_Before_At(first, 0, (-1), repl_list)
		} else {
			olen := bdata.Lines.Qty
			/* This loop is only meaningful when replacing lines.
			 * All other paths break after the first iteration. */
			for i := 0; i < iters; i++ {
				if diff > 0 && i < olen {
					diff--
					if i < len(repl_list) {
						bdata.Lines.Replace_At(first+i, repl_list[i])
					} else {
						bdata.Lines.Delete_Range_At(first+i, diff+1)
						break
					}
				} else {
					/* If the first line not being replaced
					 * (first + i) is at the end of the file, then we
					 * append. Otherwise the update is prepended. */
					if (first + i) >= bdata.Lines.Qty {
						bdata.Lines.Insert_Slice_After_At(first+i, i, (-1), to_interface(repl_list)...)
					} else {
						bdata.Lines.Insert_Slice_Before_At(first+i, i, (-1), to_interface(repl_list)...)
					}
				}
			}
		}
	} else {
		/* If the replacement list is empty then all we're doing is deleting
		 * lines. However, for some reason neovim sometimes sends updates with an
		 * empty list in which both the first and last line are the same. God
		 * knows what this is supposed to indicate. I'll just ignore them. */
		echo("Deleting starting at %d, %d lines", first, diff)
		bdata.Lines.Delete_Range_At(first, diff)
	}

	/* Neovim always considers there to be at least one line in any buffer.
	 * An empty buffer therefore must have one empty line. */
	if bdata.Lines.Qty == 0 {
		bdata.Lines.Append("")
	}
	/* If the replace list wasn't empty, set the buffer as initialized. */
	if !bdata.Initialized && !empty {
		bdata.Initialized = true
	}

	if DEBUG {
		if write_buf_updates {
			write_buf(bdata.Lines)
		}

		if !bdata.Lines.Verify_Size() {
			panic("Linked list size verification failed.")
		}

		if ctick := Nvim_buf_get_changedtick(0, int(bdata.Num)); ctick == int(bdata.Ctick) {
			if n := Nvim_buf_line_count(0, int(bdata.Num)); n != bdata.Lines.Qty {
				panic_fmt("Recorded size (%d) is incorrect, actual value"+
					" is (%d), cannot continue.", bdata.Lines.Qty, n)
			}
		}
	}
}

//========================================================================================

func id_event(event *mpack.Object) *event_id {
	name := event.Index(1).Get_Expect(mpack.G_STRING).(string)

	for i, item := range event_list {
		if name == item.name {
			return &event_list[i]
		}
	}

	panic("Failed to identify event type.")
}

func write_buf(list *ll.Linked_List) {
	echo("Writing, cur size is %d", list.Qty)
	tmpfile := Nvim_call_function(0, "tempname", mpack.G_STRING).(string)
	file := safe_fopen(tmpfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_SYNC, 0600)
	defer file.Close()

	i := 0
	for cur := list.Head; cur != nil; cur = cur.Next {
		file.WriteString(fmt.Sprintf("%d:\t%s\n", i, cur.Data.(string)))
		i++
		// file.WriteString(cur.Data.(string) + "\n")
	}

	echo("Done writing file - %s.", tmpfile)
}

func to_interface(strlist []string) []interface{} {
	ret := make([]interface{}, len(strlist))

	for i := range strlist {
		ret[i] = strlist[i]
	}

	return ret
}
