package scan

import (
	"bytes"
	// "tag_highlight/api"
)

const (
	WANT_IF_ZERO = 257
)

//========================================================================================

func (bdata *Bufdata) Strip_Comments(vimbuf []byte) []byte {
	switch bdata.Id {
	case FT_C, FT_CPP, FT_GO, FT_JAVA, FT_CSHARP:
		bte := strip_c_like(vimbuf)
		ret := bytes.Join(bte, []byte(""))
		return ret
	default:
		return vimbuf
	}
}

func strip_c_like(data []byte) [][]byte {
	var (
		ifcount int
		want    int
		esc     bool
		lines   = bytes.Split(data, []byte("\n"))
		ret     = make([][]byte, len(lines))
		n       int
	)

	for _, line := range lines {
		var (
			repl = make([]byte, len(line)+1)
			i, x int
		)

		if want == 0 {
			for i < len(line) && isblank(line[i]) {
				repl[x] = line[i]
				i++
				x++
			}
		} else {
			for i < len(line) && isblank(line[i]) {
				i++
			}
		}
		if i == len(line) {
			goto next_line
		}

		if want == WANT_IF_ZERO {
			if line[i] == '#' {
				tmp := i + 1
				for tmp < len(line) && isblank(line[tmp]) {
					tmp++
				}
				if tmp < len(line) {
					if bytes.HasPrefix(line[tmp:], []byte("if")) {
						ifcount++
					} else if bytes.HasPrefix(line[tmp:], []byte("endif")) {
						ifcount--
					}
				}

				if ifcount == 0 {
					want = 0
				}
			}
			goto next_line
		} else if is_c && want == 0 && line[i] == '#' {
			tmp := i + 1
			for tmp < len(line) && isblank(line[tmp]) {
				tmp++
			}
			if tmp < len(line) && bytes.HasPrefix(line[tmp:], []byte("if 0")) {
				ifcount++
				want = WANT_IF_ZERO
				goto next_line
			}
		}

		for ; i < len(line); i++ {
			if isblank(line[i]) {
				// nothing to do
			} else if want != 0 {
				if want == '\n' {
					if len(line) > 0 && line[len(line)-1] == '\\' {
						want = '\n'
					} else {
						want = 0
					}
					goto next_line
				}
				if int(line[i]) == want {
					switch want {
					case '*':
						if i+1 < len(line) && line[i+1] == '/' {
							want = 0
							i += 2
						}
					case '\'', '"':
						if !esc {
							want = 0
							i++
						}
					}
				}
			} else {
				switch line[i] {
				case '\'':
					want = '\''
				case '"':
					want = '"'
				case '/':
					if i+1 < len(line) {
						if line[i+1] == '*' {
							want = '*'
							repl[x] = ' '
							x++
							i++
						} else if line[i+1] == '/' {
							if line[len(line)-1] == '\\' {
								want = '\n'
							} else {
								want = 0
							}
							goto next_line
						}
					}
				}
			}

			if i < len(line) {
				if want == 0 && (!isblank(line[i]) || (i > 0 && !isblank(line[i-1]))) {
					repl[x] = line[i]
					x++
				}
				if line[i] == '\\' {
					esc = !esc
				} else {
					esc = false
				}
			}
		}

	next_line:
		esc = false
		repl[x] = '\n'
		ret[n] = repl[:x+1]
		n++
	}

	return ret
}

func isblank(ch byte) bool {
	return ch == ' ' || ch == '\t'
}
