package scan

import (
	"bytes"
	"sort"
	"tag_highlight/api"
)

type cmp_f func(byte, bool) bool

//========================================================================================

func tokenize(vimbuf []byte, id int) [][]byte {
	toks := make([][]byte, 0, 8192)

	switch id {
	case FT_VIM:
		tokenize_vim(vimbuf, &toks, vim_func)
	default:
		do_tokenize(vimbuf, &toks, c_func)
	}

	if len(toks) == 0 {
		return [][]byte{[]byte("")}
	}

	api.Echo("Got %d toks", len(toks))
	sort.Slice(toks, func(i, j int) bool { return bytes.Compare(toks[i], toks[j]) < 0 })
	ret := make([][]byte, 1, len(toks))
	ret[0] = toks[0]

	for i := 1; i < len(toks); i++ {
		if !bytes.Equal(toks[i], toks[i-1]) {
			ret = append(ret, toks[i])
		}
	}

	api.Echo("... of which %d are unique", len(ret))
	return ret
}

func do_tokenize(vimbuf []byte, toklist *[][]byte, check cmp_f) {
	var tok []byte

	for strsep(&vimbuf, &tok, check) {
		if len(tok) == 0 {
			continue
		}
		*toklist = append(*toklist, tok)
	}
}

func tokenize_vim(vimbuf []byte, toklist *[][]byte, check cmp_f) {
	var tok []byte

	for strsep(&vimbuf, &tok, check) {
		if len(tok) == 0 {
			continue
		}
		*toklist = append(*toklist, tok)

		if i := bytes.IndexByte(tok, ':'); i > 0 {
			*toklist = append(*toklist, tok[i+1:])
		}
	}
}

func strsep(strp, tok *[]byte, check cmp_f) bool {
	if strp == nil {
		return false
	}
	i := 0
	*tok = *strp

	for ; i < len(*tok) && !check((*strp)[i], true); i++ {
	}
	if i == len(*tok) {
		*tok = nil
		strp = nil
		return false
	}

	for x := i + 1; x < len(*tok); x++ {
		if !check((*strp)[x], false) {
			if len((*strp)[x:]) == 0 {
				strp = nil
			} else {
				*strp = (*tok)[x:]
				*tok = (*tok)[i:x]
			}
			break
		}
	}

	return (strp != nil)
}

//========================================================================================

func isalpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isalnum(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func c_func(ch byte, first bool) bool {
	if first {
		return ch == '_' || isalpha(ch)
	} else {
		return ch == '_' || isalnum(ch)
	}
}

func vim_func(ch byte, first bool) bool {
	if first {
		return ch == '_' || isalpha(ch)
	} else {
		return ch == '_' || ch == ':' || isalnum(ch)
	}
}
