package hn

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// Link is an <a href> found in comment or story HTML.
type Link struct {
	Text string
	URL  string
}

// RenderHTML turns HN's comment HTML (<p>, <a>, <i>, <pre><code>) into
// plain text for the terminal, and collects its links. Entities are decoded
// by the tokenizer; the result is sanitized.
func RenderHTML(src string) (string, []Link) {
	if src == "" {
		return "", nil
	}
	var b strings.Builder
	var links []Link
	seen := map[string]bool{}
	inPre := false
	var href string
	var linkText strings.Builder
	inLink := false

	paragraph := func() {
		s := strings.TrimRight(b.String(), " \n")
		b.Reset()
		b.WriteString(s)
		if s != "" {
			b.WriteString("\n\n")
		}
	}

	z := html.NewTokenizer(strings.NewReader(src))
	for {
		switch z.Next() {
		case html.ErrorToken:
			text := Sanitize(strings.TrimSpace(b.String()))
			return text, links
		case html.TextToken:
			t := string(z.Text())
			if !inPre {
				t = strings.ReplaceAll(t, "\n", " ")
			}
			b.WriteString(t)
			if inLink {
				linkText.WriteString(t)
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "p":
				paragraph()
			case "br":
				b.WriteString("\n")
			case "pre":
				paragraph()
				inPre = true
			case "a":
				href = ""
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					if string(k) == "href" {
						href = string(v)
					}
				}
				inLink = href != ""
				linkText.Reset()
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "pre":
				inPre = false
				paragraph()
			case "a":
				if inLink && !seen[href] {
					seen[href] = true
					text := Sanitize(strings.TrimSpace(linkText.String()))
					if text == "" {
						text = href
					}
					links = append(links, Link{Text: text, URL: Sanitize(href)})
				}
				inLink = false
			}
		}
	}
}

// Sanitize strips characters a terminal would interpret rather than print:
// C0/C1 control codes (ESC would let HN content inject escape sequences)
// and bidi overrides (which can make text display differently than it reads).
// Newlines survive; tabs become spaces.
func Sanitize(s string) string {
	// invalid UTF-8 first: a stray 0x9b byte is CSI on 8-bit terminals
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	clean := true
	for _, r := range s {
		if unsafeRune(r) || r == '\t' {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case unsafeRune(r):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func unsafeRune(r rune) bool {
	switch {
	case r == '\n':
		return false
	case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
		return true
	case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}
