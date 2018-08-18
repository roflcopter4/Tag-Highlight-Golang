package lists

import (
	"fmt"
	"os"
)

func eprintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}
