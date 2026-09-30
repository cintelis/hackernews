package hn

import (
	"reflect"
	"testing"
)

func TestRenderHTML(t *testing.T) {
	cases := []struct {
		name, in, want string
		links          []Link
	}{
		{"empty", "", "", nil},
		{"plain", "hello", "hello", nil},
		{"entities", "a &amp; b &#x27;c&#x27; &lt;d&gt; &#x2F;", "a & b 'c' <d> /", nil},
		{"paragraphs", "one<p>two<p>three", "one\n\ntwo\n\nthree", nil},
		{"italic", "an <i>important</i> point", "an important point", nil},
		{
			"link", `see <a href="https:&#x2F;&#x2F;example.com&#x2F;x?a=1&amp;b=2" rel="nofollow">https://example.com/x</a> ok`,
			"see https://example.com/x ok",
			[]Link{{"https://example.com/x", "https://example.com/x?a=1&b=2"}},
		},
		{
			"duplicate links collapse",
			`<a href="https://a.com">one</a> <a href="https://a.com">two</a>`,
			"one two",
			[]Link{{"one", "https://a.com"}},
		},
		{"code block keeps newlines", "look:<pre><code>  x := 1\n  y := 2\n</code></pre>after", "look:\n\n  x := 1\n  y := 2\n\nafter", nil},
		{"escape sequence stripped", "safe&#27;[2Jtext\x1b]0;title\x07", "safe[2Jtext]0;title", nil},
		{"bidi override stripped", "abc‮dcba", "abcdcba", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, links := RenderHTML(c.in)
			if got != c.want {
				t.Errorf("text = %q, want %q", got, c.want)
			}
			if !reflect.DeepEqual(links, c.links) {
				t.Errorf("links = %#v, want %#v", links, c.links)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	if got := Sanitize("a\tb\nc\x00\u009bd"); got != "a    b\ncd" {
		t.Errorf("Sanitize = %q", got)
	}
	// a raw 0x9b byte (8-bit CSI) is invalid UTF-8: it must come out as
	// U+FFFD, never as the byte itself
	if got := Sanitize("x\x9by"); got != "x�y" {
		t.Errorf("Sanitize(invalid byte) = %q", got)
	}
	s := "unchanged ✓ 日本語"
	if got := Sanitize(s); got != s {
		t.Errorf("Sanitize altered clean text: %q", got)
	}
}

func TestParseItemLink(t *testing.T) {
	cases := []struct {
		in   string
		want ItemRef
		ok   bool
	}{
		{"https://news.ycombinator.com/item?id=123", ItemRef{ID: 123}, true},
		{"https://news.ycombinator.com/item?id=123#456", ItemRef{ID: 123, Anchor: 456}, true},
		{"http://www.news.ycombinator.com/item?id=9", ItemRef{ID: 9}, true},
		{"https://news.ycombinator.com/item?id=abc", ItemRef{}, false},
		{"https://news.ycombinator.com/item?id=-4", ItemRef{}, false},
		{"https://news.ycombinator.com/user?id=pg", ItemRef{}, false},
		{"https://evil.com/item?id=1", ItemRef{}, false},
		{"javascript:alert(1)", ItemRef{}, false},
	}
	for _, c := range cases {
		got, ok := ParseItemLink(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseItemLink(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestReplyURL(t *testing.T) {
	want := "https://news.ycombinator.com/reply?id=456&goto=item%3Fid%3D123%23456"
	if got := ReplyURL(123, 456); got != want {
		t.Errorf("ReplyURL = %q, want %q", got, want)
	}
}
