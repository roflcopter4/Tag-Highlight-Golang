package lists

import (
	"bytes"
	"log"
	"tag_highlight/util"
)

type Node struct {
	Data interface{}
	Next *Node
	Prev *Node
}

type Linked_List struct {
	Head *Node
	Tail *Node
	Qty  int
}

//========================================================================================

func New_List() *Linked_List {
	list := Linked_List{
		Head: nil,
		Tail: nil,
		Qty:  0,
	}
	return &list
}

func (list *Linked_List) Prepend(data interface{}) {
	var node Node

	if list.Head != nil {
		list.Head.Prev = &node
	}
	if list.Tail == nil {
		list.Tail = &node
	}

	node.Data = data
	node.Prev = nil
	node.Next = list.Head
	list.Head = &node

	list.Qty++
}

func (list *Linked_List) Append(data interface{}) {
	var node Node

	if list.Tail != nil {
		list.Tail.Next = &node
	}
	if list.Head == nil {
		list.Head = &node
	}

	node.Data = data
	node.Prev = list.Tail
	node.Next = nil
	list.Tail = &node

	list.Qty++
}

func (list *Linked_List) Join(str []byte) []byte {
	/* if list == nil || list.Qty == 0 {
		return ""
	}
	var buf string */
	var buf bytes.Buffer
	buf.Grow(0x4000)

	for node := list.Head; node != nil; node = node.Next {
		// buf += node.Data.(string) + str
		buf.WriteString(node.Data.(string))
		buf.Write(str)
	}

	/* for node := list.Head; node != nil; node = node.Next {
		switch list.Head.Data.(type) {
		case string:
			buf += node.Data.(string) + str
		case *string:
			buf += *node.Data.(*string) + str
		case []byte:
			buf += string(node.Data.([]byte)) + str
		case *[]byte:
			buf += string(*node.Data.(*[]byte)) + str
		case []rune:
			buf += string(node.Data.([]rune)) + str
		case *[]rune:
			buf += string(*node.Data.(*[]rune)) + str
		default:
			// panic("Node data type is not a string type I can recognize: cannot join.")
		}
	} */

	return buf.Bytes()
}

func (list *Linked_List) MakeSliceStr() []string {
	ret := make([]string, list.Qty)
	i := 0
	for node := list.Head; node != nil; node = node.Next {
		ret[i] = node.Data.(string)
		i++
	}
	return ret
}

//========================================================================================

func (list *Linked_List) Insert_After(at *Node, data interface{}) {
	var node Node
	node.Data = data
	node.Prev = at

	if at != nil {
		node.Next = at.Next
		at.Next = &node
		if node.Next != nil {
			node.Next.Prev = &node
		}
	} else {
		node.Next = nil
	}

	if list.Head == nil {
		list.Head = &node
	}
	if list.Tail == nil || at == list.Tail {
		list.Tail = &node
	}

	list.Qty++
}

func (list *Linked_List) Insert_Before(at *Node, data interface{}) {
	var node Node
	node.Data = data
	node.Next = at

	if at != nil {
		node.Prev = at.Prev
		at.Prev = &node
		if node.Prev != nil {
			node.Prev.Next = &node
		}
	} else {
		node.Prev = nil
	}

	if list.Head == nil || at == list.Head {
		list.Head = &node
	}
	if list.Tail == nil {
		list.Tail = &node
	}

	list.Qty++
}

//========================================================================================

func resolve_neg(val, base int) int {
	if val >= 0 {
		return val
	} else {
		return val + base + 1
	}
}

// func sanity_check1(data []interface{}) {
//         if start < 0 || end < 1 || start == end {
//                 log.Panicf("Illegal start (%d) and end (%d)", start, end)
//         }
//         if start > end {
//                 log.Panicf("End (%d) cannot be greater than start (%d)", start, end)
//         }
//         if data == nil || len(data) == 0 {
//                 panic("No data supplied")
//         }
//         if end < len(data) {
//                 log.Panicf("End (%d) cannot be greater than the length of data (%d)", end, len(data))
//         }
// }

func (list *Linked_List) create_nodes(i int, tmp []*Node, data []interface{}) int {
	for x := 1; x < len(data); x++ {
		tmp[i] = &Node{}
		tmp[i].Data = data[x]
		tmp[i].Prev = tmp[i-1]
		tmp[i-1].Next = tmp[i]

		// list.Qty++
		i++
	}

	return i
}

func (list *Linked_List) Insert_Slice_After(at *Node, data ...interface{}) {
	// start = resolve_neg(start, int(len(data)))
	// end = resolve_neg(end, int(len(data)))
	// sanity_check1(start, end, data)

	// diff := end - start
	diff := len(data)
	// util.Eprintf("Len: %d, start: '%v'\n", len(data), at)
	if diff == 1 {
		list.Insert_After(at, data[0])
		return
	}

	tmp := make([]*Node, diff)
	tmp[0] = &Node{}
	tmp[0].Data = data[0]
	tmp[0].Prev = at
	// list.Qty++

	last := list.create_nodes(1, tmp, data) - 1

	if at != nil {
		tmp[last].Next = at.Next
		at.Next = tmp[0]
		if tmp[last].Next != nil {
			tmp[last].Next.Prev = tmp[last]
		}
	} else {
		tmp[last].Next = nil
	}

	if list.Head == nil {
		list.Head = tmp[0]
	}
	if list.Tail == nil || at == list.Tail {
		list.Tail = tmp[last]
	}

	list.Qty += diff
}

func (list *Linked_List) Insert_Slice_Before(at *Node, data ...interface{}) {
	// start = resolve_neg(start, int(len(data)))
	// end = resolve_neg(end, int(len(data)))
	// sanity_check1(start, end, data)

	// diff := end - start
	diff := len(data)
	// util.Eprintf("Len: %d, start: '%v'\n", len(data), at)
	if diff == 1 {
		list.Insert_Before(at, data[0])
		return
	}

	tmp := make([]*Node, diff)
	tmp[0] = &Node{}
	tmp[0].Data = data[0]
	// list.Qty++

	last := list.create_nodes(1, tmp, data) - 1
	tmp[last].Next = at

	if at != nil {
		tmp[0].Prev = at.Prev
		at.Prev = tmp[last]
		if tmp[0].Prev != nil {
			tmp[0].Prev.Next = tmp[0]
		}
	} else {
		tmp[0].Prev = nil
	}

	if list.Head == nil || at == list.Head {
		list.Head = tmp[0]
	}
	if list.Tail == nil {
		list.Tail = tmp[last]
	}

	list.Qty += diff
}

//========================================================================================

func (list *Linked_List) Delete_Node(node *Node) {
	if list.Qty == 1 {
		list.Head, list.Tail = nil, nil
	} else if node == list.Head {
		list.Head = node.Next
		list.Head.Prev = nil
	} else if node == list.Tail {
		list.Tail = node.Prev
		list.Tail.Next = nil
	} else {
		node.Prev.Next = node.Next
		node.Next.Prev = node.Prev
	}

	list.Qty--
}

func (list *Linked_List) Delete_Range(at *Node, rng int) {
	// util.Eprintf("Deleting range %d\n", rng)
	if list.Qty < rng {
		log.Panicf("Delete range (%d) cannot be larger than the list's size (%d)", rng, list.Qty)
	}

	if rng == 0 {
		return
	}
	/* if rng == 1 {
		list.Delete_Node(at)
	} */

	var (
		/* start        *Node = at.Prev
		end          *Node = nil */
		last         *Node = nil
		current      *Node = at
		next         *Node = nil
		prev         *Node = nil
		replace_head bool  = (at == list.Head || list.Head == nil)
	)

	if at != nil {
		prev = at.Prev
	}

	for i := 0; i < rng && current != nil; i++ {
		/* next = current.Next
		current = next */
		last = current
		current = current.Next
		// list.Qty--
	}
	next = current

	if list.Qty < 0 {
		list.Qty = 0
	}

	if replace_head {
		list.Head = next
	}
	if prev != nil {
		prev.Next = next
	}
	if next != nil {
		next.Prev = prev
	} else {
		last.Prev = prev
		list.Tail = prev
	}

	// util.Eprintf("qty is %d\n", list.Qty)
	list.Qty -= rng
	// util.Eprintf("qty is %d\n", list.Qty)
}

//========================================================================================

/*
 * These were all macros in the original C code, but they should be inlined in
 * pretty much every instance anyway.
 */
func (list *Linked_List) Insert_Before_At(index int, data interface{}) {
	list.Insert_Before(list.At(index), data)
}

func (list *Linked_List) Insert_After_At(index int, data interface{}) {
	list.Insert_After(list.At(index), data)
}

func (list *Linked_List) Insert_Slice_Before_At(index int, data ...interface{}) {
	list.Insert_Slice_Before(list.At(index), data...)
}

func (list *Linked_List) Insert_Slice_After_At(index int, data ...interface{}) {
	list.Insert_Slice_After(list.At(index), data...)
}

func (list *Linked_List) Delete_Node_At(index int) {
	list.Delete_Node(list.At(index))
}

func (list *Linked_List) Delete_Range_At(index, rng int) {
	list.Delete_Range(list.At(index), rng)
}

//========================================================================================

func (list *Linked_List) At(index int) *Node {
	if list == nil || list.Qty == 0 || list.Head == nil || list.Tail == nil {
		return nil
	}
	if index == 0 {
		return list.Head
	}
	if index == (-1) || index == list.Qty {
		return list.Tail
	}

	index = resolve_neg(index, list.Qty)
	if index < 0 || index > list.Qty {
		util.Eprintf("Warning: Cannot find node at index %d (list qty: %d)\n",
			index, list.Qty)
	}

	var (
		current *Node = nil
		x       int
	)

	if index < ((list.Qty - 1) / 2) {
		current = list.Head
		for x = 0; current != nil && x != index; x++ {
			// util.Eprintf("x: %d -> %v\n", x, current.Data)
			current = current.Next
		}
	} else {
		current = list.Tail
		for x = (list.Qty - 1); current != nil && x != index; x-- {
			// util.Eprintf("x: %d -> %v\n", x, current.Data)
			current = current.Prev
		}
	}
	if x != index {
		log.Panicf("Couldn't find node at index %d", index)
	}

	return current
}

func (list *Linked_List) Replace_At(index int, data interface{}) {
	node := list.At(index)
	if node == nil {
		panic("Node out of range!")
	}

	node.Data = data
}

func (list *Linked_List) Verify_Size() bool {
	i := 0
	for node := list.Head; node != nil; i++ {
		node = node.Next
	}
	ret := i == list.Qty
	if !ret {
		util.Eprintf("Size %d is not correct (%d)\n", list.Qty, i)
		list.Qty = i
	}
	return ret
}
