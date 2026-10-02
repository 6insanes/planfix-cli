// Package output renders command results as tables, detail blocks, or JSON.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// Table writes a tab-aligned table with a header row. An empty cols
// argument omits the header line.
func Table(w io.Writer, cols []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(cols) > 0 {
		for i, c := range cols {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, c)
		}
		fmt.Fprintln(tw)
	}
	for _, row := range rows {
		for i, c := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, c)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
}

// Detail writes a key/value block with aligned values.
func Detail(w io.Writer, kv [][2]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, pair := range kv {
		fmt.Fprintf(tw, "%s:\t%s\n", pair[0], pair[1])
	}
	tw.Flush()
}

// JSON pretty-prints a raw API response body. If raw is not valid JSON it
// is written through unchanged.
func JSON(w io.Writer, raw []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		_, werr := w.Write(raw)
		return werr
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}
