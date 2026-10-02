package output

import (
	"fmt"
	"io"
)

// WriteOpts selects the rendering mode for WriteResult.
type WriteOpts struct {
	JSON  bool
	Quiet bool
}

// WriteResult renders a create/update/add outcome: pretty JSON body when
// opts.JSON, the bare id when opts.Quiet, otherwise "verb noun id".
func WriteResult(w io.Writer, opts WriteOpts, verb, noun string, id int, raw []byte) error {
	switch {
	case opts.JSON:
		return JSON(w, raw)
	case opts.Quiet:
		fmt.Fprintln(w, id)
	default:
		fmt.Fprintf(w, "%s %s %d\n", verb, noun, id)
	}
	return nil
}
