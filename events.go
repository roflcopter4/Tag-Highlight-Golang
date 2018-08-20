package main

import (
	"fmt"
	"os"
	// "runtime/pprof"
	"sync"
	"tag_highlight/api"
	"tag_highlight/lists"
	"tag_highlight/mpack"
	"tag_highlight/util"
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
			api.Echo("Detaching from buffer %d", bufnum)
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
		iters     = util.Max_Int(diff, len(repl_list))
		empty     = false
	)

	insert_slice := func(i int) {
		/* If the first line not being replaced (first + i) is at the end
		 * of the file, then we append. Otherwise the update is prepended. */
		if (first) >= bdata.Lines.Qty {
			bdata.Lines.Insert_Slice_After_At(first+i, sl2i(repl_list[i:])...)
		} else {
			bdata.Lines.Insert_Slice_Before_At(first+i, sl2i(repl_list[i:])...)
		}
	}

	if len(repl_list) > 0 {
		if last == (-1) {
			panic("WHAA")
		} else if bdata.Lines.Qty <= 1 && first == 0 && len(repl_list) == 1 && len(repl_list[0]) == 0 {
			/* Useless update, one empty string in an empty buffer. Just ignore it. */
			empty = true
		} else if first == 0 && last == 0 {
			/* Inserting above the first line in the file. */
			bdata.Lines.Insert_Slice_Before_At(first, sl2i(repl_list)...)
		} else if first == last {
			insert_slice(0)
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
					insert_slice(i)
					break
				}
			}
		}
	} else {
		/* If the replacement list is empty then all we're doing is deleting
		 * lines. However, for some reason neovim sometimes sends updates with an
		 * empty list in which both the first and last line are the same. God
		 * knows what this is supposed to indicate. I'll just ignore them. */
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
			write_lines(bdata.Lines)
		}

		if !bdata.Lines.Verify_Size() {
			panic("Linked list size verification failed.")
		}

		if ctick := api.Nvim_buf_get_changedtick(0, int(bdata.Num)); ctick == int(bdata.Ctick) {
			if n := api.Nvim_buf_line_count(0, int(bdata.Num)); n != bdata.Lines.Qty {
				panic(fmt.Sprintf("Recorded size (%d) is incorrect, actual value"+
					" is (%d), cannot continue.", bdata.Lines.Qty, n))
			}
		}
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
	api.Echo("Writing, cur size is %d", list.Qty)
	tmpfile := api.Nvim_call_function(0, []byte("tempname"), mpack.E_STRING).(string)
	file := util.Safe_Fopen(tmpfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_SYNC, 0600)
	defer file.Close()

	i := 0
	for cur := list.Head; cur != nil; cur = cur.Next {
		file.WriteString(fmt.Sprintf("%d:\t%s\n", i, cur.Data.(string)))
		i++
	}

	api.Echo("Done writing file - %s.", tmpfile)
}

func write_buf(buf *[]string) {
	api.Echo("Writing, cur size is %d", len(*buf))
	tmpfile := api.Nvim_call_function(0, []byte("tempname"), mpack.E_STRING).(string)
	file := util.Safe_Fopen(tmpfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
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
	api.Echo("Recieved \"%c\", waking up!", val)

	switch val {
	case 'A', 'D':
		// tv1 := time.Now()
		// fsleep(0.08)
		timer := util.NewTimer()

		prev := bufnum
		bufnum = api.Nvim_get_current_buf(0)
		bdata := Find_Buffer(bufnum)

		if bdata == nil {
			do_attach(bufnum)
		} else if prev != bufnum {
			if bdata.Calls == nil {
				bdata.Get_Initial_Taglist()
			}
			bdata.Update_Highlight()
		} else {
			api.Echo("Prev and bufnum are the same, doing nothing. (%d and %d)", prev, bufnum)
			break
		}

		// tv2 := time.Now()
		// util.Warn("Initialization/pdate time: %f", util.Tdiff(&tv1, &tv2))
		timer.EchoReport("initialization/update")

	case 'B', 'F':
		// fsleep(0.08)
		// tv1 := time.Now()
		timer := util.NewTimer()
		bufnum = api.Nvim_get_current_buf(0)
		bdata := Find_Buffer(bufnum)

		if bdata == nil {
			api.Echo("Failed to find buffer %d", bufnum)
			do_attach(bufnum)
		} else if bdata.Update_Taglist(util.Boolint(val == 'F')) {
			bdata.Update_Highlight()
		} else {
			break
		}

		// tv2 := time.Now()
		// util.Warn("Update time: %f", util.Tdiff(&tv1, &tv2))
		timer.EchoReport("update")

	case 'C':
		// TODO clear highlight
		// TODO kill parent id and/or gracefully die

		// pprof.StopCPUProfile()
		os.Exit(0)

	case 'E':
		// TODO clear highlight
		fallthrough

	default:
		api.Echo("Hmm, nothing to do...")
	}
}

func do_attach(bnum int) *Bufdata {
	bdata := New_Buffer(0, bufnum)
	timer := util.NewTimer()

	if bdata != nil {
		api.Nvim_buf_attach(1, bufnum)

		bdata.get_initial_lines()
		bdata.Get_Initial_Taglist()
		bdata.Update_Highlight()
	}
	timer.EchoReport("attaching")

	return bdata
}
