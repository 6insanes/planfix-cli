package output

import "testing"

func TestStripHTMLRemovesMarkup(t *testing.T) {
	got := StripHTML(`<p>Hello <b>world</b></p><h2>Title</h2>`)
	want := "Hello world\n\nTitle"
	if got != want {
		t.Errorf("StripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTMLBlockTagsBecomeNewlines(t *testing.T) {
	got := StripHTML("line1<br>line2</p><p>line3</p>")
	if got != "line1\nline2\n\nline3" {
		t.Errorf("StripHTML() = %q", got)
	}
}

func TestStripHTMLStyleBlockDropped(t *testing.T) {
	got := StripHTML(`<style>h1{color:red}</style><p>text</p>`)
	if got != "text" {
		t.Errorf("StripHTML() = %q, want %q", got, "text")
	}
}

func TestStripHTMLListItemsGetBullets(t *testing.T) {
	got := StripHTML("<ul><li>one</li><li>two</li></ul>")
	want := "- one\n- two"
	if got != want {
		t.Errorf("StripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTMLTableCellsJoin(t *testing.T) {
	got := StripHTML("<table><tr><td>a</td><td>b</td></tr></table>")
	want := "a b"
	if got != want {
		t.Errorf("StripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTMLSourceIndentationJoinsCells(t *testing.T) {
	got := StripHTML("<table>\n <tr>\n  <td>a</td>\n  <td>b</td>\n </tr>\n</table>")
	if got != "a b" {
		t.Errorf("StripHTML() = %q, want %q", got, "a b")
	}
}

func TestStripHTMLNewlineBetweenInlineTagsKeepsWordsApart(t *testing.T) {
	got := StripHTML("<p><span>foo</span>\n<span>bar</span></p>")
	if got != "foo bar" {
		t.Errorf("StripHTML() = %q, want %q", got, "foo bar")
	}
}

func TestStripHTMLDecodesEntities(t *testing.T) {
	got := StripHTML("a&nbsp;b &amp; &lt;tag&gt;")
	want := "a b & <tag>"
	if got != want {
		t.Errorf("StripHTML() = %q, want %q", got, want)
	}
}

func TestStripHTMLKeepsComparisonText(t *testing.T) {
	got := StripHTML("count < limit && flag > 0")
	if got != "count < limit && flag > 0" {
		t.Errorf("StripHTML() = %q", got)
	}
}

func TestStripHTMLCollapsesBlankLines(t *testing.T) {
	got := StripHTML("<p>a</p>\n\n\n\n<p>b</p>")
	if got != "a\n\nb" {
		t.Errorf("StripHTML() = %q, want %q", got, "a\n\nb")
	}
}

func TestStripHTMLPlainTextUnchanged(t *testing.T) {
	const in = "just text"
	if got := StripHTML(in); got != in {
		t.Errorf("StripHTML() = %q, want %q", got, in)
	}
}

func TestStripRowsNoPlainKeepsHTML(t *testing.T) {
	rows := [][]string{{"<p>x</p>"}}
	if got := StripRows(rows, false); got[0][0] != "<p>x</p>" {
		t.Errorf("StripRows(plain=false) = %q, want input unchanged", got[0][0])
	}
}

func TestStripRowsPlainStrips(t *testing.T) {
	rows := [][]string{{"<p>x</p>", "id"}}
	got := StripRows(rows, true)
	if got[0][0] != "x" || got[0][1] != "id" {
		t.Errorf("StripRows(plain=true) = %q, want [x id]", got[0])
	}
}

func TestStripKVPlainStripsValuesOnly(t *testing.T) {
	kv := [][2]string{{"NAME", "<b>Ship</b>"}, {"ID", "42"}}
	got := StripKV(kv, true)
	if got[0][1] != "Ship" || got[0][0] != "NAME" || got[1][1] != "42" {
		t.Errorf("StripKV() = %v", got)
	}
}

func TestStripKVNoPlainKeepsValues(t *testing.T) {
	kv := [][2]string{{"NAME", "<b>Ship</b>"}}
	if got := StripKV(kv, false); got[0][1] != "<b>Ship</b>" {
		t.Errorf("StripKV(plain=false) = %q", got[0][1])
	}
}
