package api

//========================================================================================

func (list *Atomic_list) Nvim_command(cmd []byte) {
	call := Atomic_call{}
	call.Args = make([]interface{}, 2)
	call.Args[0] = []byte("nvim_command")
	call.Args[1] = cmd
	call.Fmt = "c[c]"

	list.Calls = append(list.Calls, call)
}
