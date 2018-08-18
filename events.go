package main

import (
	"fmt"
	"os"
	"sync"
	"tag_highlight/lists"
	"tag_highlight/mpack"
	"time"
)

const ( // Event types
	event_BUF_LINES = iota
	event_BUF_CHANGED_TICK
	event_BUF_DETACH
	event_VIM_UPDATE
)

// const write_buf_updates = true
const write_buf_updates = false

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
		go interrupt_call(rune(info.Index(0).Expect(mpack.E_STRING).(string)[0]))
	} else {
		bufnum := int(info.Index(0).Expect(mpack.T_NUM).(int64))
		bdata := Find_Buffer(bufnum)
		if bdata == nil {
			panic("Update called on an uninitialized buffer.")
		}

		switch etype.id {
		case event_BUF_LINES:
			handle_line_event(bdata, info)
		case event_BUF_CHANGED_TICK:
			bdata.Ctick = uint32(info.Index(1).Expect(mpack.T_NUM).(int64))
		case event_BUF_DETACH:
			echo("Detaching from buffer %d", bufnum)
			Remove_Buffer(bufnum)
		}
	}
}

//========================================================================================

func handle_line_event(bdata *Bufdata, data *mpack.Object) {
	if data.Index(5).Expect(mpack.T_BOOL).(bool) {
		panic("Somehow the boolean argument is true!")
	}
	if write_buf_updates {
		write_lines(bdata.Lines)
	}

	bdata.Ctick = uint32(data.Index(1).Expect(mpack.T_NUM).(int64))
	var (
		first     = int(data.Index(2).Expect(mpack.T_NUM).(int64))
		last      = int(data.Index(3).Expect(mpack.T_NUM).(int64))
		repl_list = data.Index(4).Expect(mpack.E_STRLIST).([]string)
		diff      = last - first
		iters     = max_int(diff, len(repl_list))
		empty     = false
	)

	ass := func(i int) {
		if (first) >= bdata.Lines.Qty {
			echo("(%d): Inserting lst after at pos %d, len %d", bdata.Lines.Qty, first+i, len(repl_list))
			bdata.Lines.Insert_Slice_After_At(first+i, sl2i(repl_list[i:])...)
		} else {
			echo("(%d): Inserting lst before at pos %d, len %d", bdata.Lines.Qty, first+i+1, len(repl_list))
			bdata.Lines.Insert_Slice_Before_At(first+i, sl2i(repl_list[i:])...)
		}
	}

	if len(repl_list) > 0 {
		if last == (-1) {
			/* Initial lines update. */
			// if bdata.Lines.Qty == 1 {
			//         bdata.Lines.Delete_Node(bdata.Lines.Head)
			//         assert(bdata.Lines.Verify_Size(), "Verify failed.")
			// }
			// bdata.Lines.Insert_Slice_After(bdata.Lines.Head, 0, (-1), sl2i(repl_list)...)
			panic("WHAA")
		} else if bdata.Lines.Qty <= 1 && first == 0 && len(repl_list) == 1 && len(repl_list[0]) == 0 {
			/* Useless update, one empty string in an empty buffer.
			 * Just ignore it. */
			echo("Empty update, ignoring.")
			empty = true
		} else if first == 0 && last == 0 {
			/* Inserting above the first line in the file. */
			echo("(%d): Inserting slice len %d before start", bdata.Lines.Qty)
			bdata.Lines.Insert_Slice_Before_At(first, sl2i(repl_list)...)
			// echo("(%d): Inserting slice len %d before start", len(repl_list))
			// bdata.Buf = append(repl_list, bdata.Buf...)
		} else if first == last {
			echo("Jumping to insert...")
			ass(0)
		} else {
			olen := bdata.Lines.Qty
			/* This loop is only meaningful when replacing lines.
			 * All other paths break after the first iteration. */
			for i := 0; i < iters; i++ {
				echo("Diff is %d", diff)
				if diff > 0 && i < olen {
					diff--
					if i < len(repl_list) {
						// echo("(%d): Replacing line %d", bdata.Lines.Qty, i)
						bdata.Lines.Replace_At(first+i, repl_list[i])
						// echo("(%d): Replacing line %d", len(bdata.Buf), i)
						// bdata.Buf[first+i] = repl_list[i]
					} else {
						echo("(%d): Deleting range %d to %d (%d lines)", bdata.Lines.Qty, first+i, first+i+diff+1, diff)
						bdata.Lines.Delete_Range_At(first+i, diff+1)
						// echo("(%d): Deleting range %d to %d (%d lines)", len(bdata.Buf), first+i, first+i+diff+1, diff)
						// bdata.Buf = append(bdata.Buf[:first+i], bdata.Buf[first+i+diff+1:]...)
						break
					}
				} else {
					/* If the first line not being replaced
					 * (first + i) is at the end of the file, then we
					 * append. Otherwise the update is prepended. */
					ass(i)
					break
					// if (first + i) >= bdata.Lines.Qty {
					//         echo("(%d): Inserting lst after at pos %d, len %d", bdata.Lines.Qty, first+i, len(repl_list))
					//         // bdata.Lines.Insert_Slice_After_At(first+i, i, (-1), sl2i(repl_list)...)
					//         bdata.Lines.Insert_Slice_After_At(first+i, sl2i(repl_list[i:])...)
					//         [> echo("(%d): Inserting lst after at pos %d, len %d", len(bdata.Buf), first+i, len(repl_list)) <]
					//         // tmp := bdata.Buf[first+i:]
					//         // bdata.Buf = append(bdata.Buf[:first+i+1], repl_list[i:]...)
					//         // bdata.Buf = append(bdata.Buf, tmp...)
					// } else {
					//         echo("(%d): Inserting lst before at pos %d, len %d", bdata.Lines.Qty, first+i+1, len(repl_list))
					//         bdata.Lines.Insert_Slice_Before_At(first+i, sl2i(repl_list[i:])...)
					//         // echo("(%d): Inserting lst before at pos %d, len %d", len(bdata.Buf), first+i+1, len(repl_list))
					//         // tmp := bdata.Buf[:first+i+1]
					//         // tmp = append(tmp, bdata.Buf[i:]...)
					//         // bdata.Buf = append(tmp, bdata.Buf[:first+i]...)
					// }
				}
			}
		}
	} else {
		/* If the replacement list is empty then all we're doing is deleting
		 * lines. However, for some reason neovim sometimes sends updates with an
		 * empty list in which both the first and last line are the same. God
		 * knows what this is supposed to indicate. I'll just ignore them. */
		echo("(%d): Deleting from %d to %d, (%d lines)", bdata.Lines.Qty, first, first+diff, diff)
		bdata.Lines.Delete_Range_At(first, diff)
		// echo("(%d): Deleting from %d to %d, (%d lines)", len(bdata.Buf), first, first+diff, diff)
		// if first > 0 {
		//         tmp := bdata.Buf[:first]
		//         bdata.Buf = append(tmp, bdata.Buf[first+1:]...)
		// } else {
		//         bdata.Buf = bdata.Buf[:last]
		// }
	}

	/* Neovim always considers there to be at least one line in any buffer.
	 * An empty buffer therefore must have one empty line. */
	if bdata.Lines.Qty == 0 {
		// if len(bdata.Buf) == 0 {
		bdata.Lines.Append("")
		// bdata.Buf = append(bdata.Buf, "")
	}
	/* If the replace list wasn't empty, set the buffer as initialized. */
	if !bdata.Initialized && !empty {
		bdata.Initialized = true
	}

	if DEBUG {
		if write_buf_updates {
			write_lines(bdata.Lines)
			// write_buf(&bdata.Buf)
		}

		if !bdata.Lines.Verify_Size() {
			panic("Linked list size verification failed.")
		}

		if ctick := Nvim_buf_get_changedtick(0, int(bdata.Num)); ctick == int(bdata.Ctick) {
			if n := Nvim_buf_line_count(0, int(bdata.Num)); n != bdata.Lines.Qty {
				panic(fmt.Sprintf("Recorded size (%d) is incorrect, actual value"+
					" is (%d), cannot continue.", bdata.Lines.Qty, n))
			}
		}
		// if ctick := Nvim_buf_get_changedtick(0, int(bdata.Num)); ctick == int(bdata.Ctick) {
		//         if n := Nvim_buf_line_count(0, int(bdata.Num)); n != len(bdata.Buf) {
		//                 log.Panicf("Recorded size (%d) is incorrect, actual value"+
		//                         " is (%d), cannot continue.", len(bdata.Buf), n)
		//         }
		// }
	}
}

//========================================================================================

func id_event(event *mpack.Object) *event_id {
	name := event.Index(1).Expect(mpack.E_STRING).(string)

	for i, item := range event_list {
		if name == item.name {
			return &event_list[i]
		}
	}

	panic("Failed to identify event type.")
}

func write_lines(list *lists.Linked_List) {
	echo("Writing, cur size is %d", list.Qty)
	tmpfile := Nvim_call_function(0, "tempname", mpack.E_STRING).(string)
	file := safe_fopen(tmpfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_SYNC, 0600)
	defer file.Close()

	i := 0
	for cur := list.Head; cur != nil; cur = cur.Next {
		file.WriteString(fmt.Sprintf("%d:\t%s\n", i, cur.Data.(string)))
		i++
	}

	echo("Done writing file - %s.", tmpfile)
}

func write_buf(buf *[]string) {
	echo("Writing, cur size is %d", len(*buf))
	tmpfile := Nvim_call_function(0, "tempname", mpack.E_STRING).(string)
	file := safe_fopen(tmpfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	defer file.Close()

	for i, cur := range *buf {
		file.WriteString(fmt.Sprintf("%d:\t%s\n", i, cur))
	}
}

func sl2i(list []string) []interface{} {
	ret := make([]interface{}, len(list))

	for i := range list {
		ret[i] = list[i]
	}

	return ret
}

//========================================================================================

var (
	bufnum int = (-1)
)

func interrupt_call(val rune) {
	echo("Recieved \"%c\", waking up!", val)

	switch val {
	case 'A', 'D':
		tv1 := time.Now()
		// fsleep(0.08)
		prev := bufnum
		bufnum = Nvim_get_current_buf(0)
		bdata := Find_Buffer(bufnum)

		if bdata == nil {
			do_attach(bufnum)
			warn("Initialization time: %f", (float64(time.Since(tv1)))/float64(time.Second))
		} else if prev != bufnum {
			if bdata.Calls == nil {
				bdata.Get_Initial_Taglist()
			}
			bdata.Update_Highlight()
			warn("Update time: %f", (float64(time.Since(tv1)))/float64(time.Second))
		} else {
			echo("Prev and bufnum are the same, doing nothing. (%d and %d)", prev, bufnum)
		}

	case 'B':
		// fsleep(0.08)
		tv1 := time.Now()
		bufnum = Nvim_get_current_buf(0)
		bdata := Find_Buffer(bufnum)

		if bdata == nil {
			echo("Failed to find buffer %d", bufnum)
			do_attach(bufnum)
			break
		}

		if bdata.Update_Taglist() {
			bdata.Update_Highlight()
			warn("Update time: %f", (float64(time.Since(tv1)))/float64(time.Second))
		}

	case 'C':
		// clear highlight
		// kill parent id
		fallthrough

	case 'E':
		// clear highlight
		fallthrough

	default:
		echo("Hmm, nothing to do...")
	}
}

func do_attach(bnum int) *Bufdata {
	bdata := New_Buffer(0, bufnum)

	if bdata != nil {
		Nvim_buf_attach(1, bufnum)

		bdata.get_initial_lines()
		bdata.Get_Initial_Taglist()
		bdata.Update_Highlight()
	}

	return bdata
}
