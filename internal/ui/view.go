package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/cintelis/hackernews/internal/app"
	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/keymap"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type seg struct {
	s    string
	fg   lipgloss.Color
	bold bool
}

func (m *Model) theme() Theme {
	if m.s.Light {
		return light
	}
	return dark
}

func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.width < 40 || m.height < 12 {
		return "terminal too small for cintelis"
	}
	t := m.theme()
	h := m.bodyHeight()

	var body []string
	switch m.s.Mode() {
	case app.ModeHelp:
		body = m.helpBody(t, h)
	case app.ModeLinks:
		body = m.linksBody(t, h)
	case app.ModeDetail:
		body = m.detailBody(t, h)
	case app.ModeError:
		body = m.errorBody(t, h)
	default:
		body = m.listBody(t, h)
	}
	for len(body) < h {
		body = append(body, line(m.width, t.Body))
	}
	out := append(m.header(t), body[:h]...)
	return strings.Join(append(out, m.status(t)...), "\n")
}

// line renders segments on one background and pads (or truncates) to w.
func line(w int, bg lipgloss.Color, segs ...seg) string {
	var b strings.Builder
	for _, sg := range segs {
		b.WriteString(lipgloss.NewStyle().Foreground(sg.fg).Background(bg).Bold(sg.bold).Render(sg.s))
	}
	s := b.String()
	n := lipgloss.Width(s)
	if n > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", w-n))
}

// split renders left and right segment groups on one line.
func split(w int, bg lipgloss.Color, left, right []seg) string {
	rw := 0
	for _, sg := range right {
		rw += lipgloss.Width(sg.s)
	}
	if rw == 0 || rw >= w {
		return line(w, bg, left...)
	}
	return line(w-rw, bg, left...) + line(rw, bg, right...)
}

func centered(w int, bg lipgloss.Color, s string, fg lipgloss.Color, bold bool) string {
	gap := max(0, (w-lipgloss.Width(s))/2)
	return line(w, bg, seg{s: strings.Repeat(" ", gap)}, seg{s: s, fg: fg, bold: bold})
}

func (m *Model) header(t Theme) []string {
	w := m.width
	// the Y tile has its own background, so it's rendered apart from the strip
	tile := lipgloss.NewStyle().Foreground(t.BrandFg).Background(t.BrandBg).Bold(true).Render(" CA ")
	brand := []seg{{" CISO", t.Brand, true}, {" AI", t.BrandAccent, true}, {"  ·  Hacker News", t.BrandSubtle, false}}
	var right []seg
	if d := len(m.s.Stack); d > 0 && m.s.Screen != app.ScreenList {
		right = append(right, seg{fmt.Sprintf("‹ %d back  ", d), t.BrandSubtle, false})
	}
	right = append(right, seg{versionLabel(m.version) + " ", t.BrandSubtle, false})
	first := line(1, t.Strip) + tile + split(w-5, t.Strip, brand, right)

	var second string
	if m.s.Screen == app.ScreenList {
		second = line(1, t.Strip)
		used := 1
		for _, c := range app.Categories {
			label := " " + c.Label() + " "
			if c == m.s.Category {
				second += lipgloss.NewStyle().Foreground(t.TabActiveFg).Background(t.TabActiveBg).Bold(true).Render(label)
			} else {
				second += lipgloss.NewStyle().Foreground(t.TabFg).Background(t.Strip).Render(label)
			}
			second += line(1, t.Strip)
			used += len(label) + 1
		}
		if used < w {
			second += line(w-used, t.Strip)
		} else {
			second = ansi.Truncate(second, w, "")
		}
	} else if m.s.Detail != nil {
		second = line(w, t.Strip, seg{" " + m.s.Detail.Story.Title, t.TabFg, true})
	} else {
		second = line(w, t.Strip)
	}
	return []string{first, second, line(w, t.Strip, seg{strings.Repeat("─", w), t.Border, false})}
}

func (m *Model) status(t Theme) []string {
	w := m.width
	var left []seg
	if m.s.Busy() {
		left = append(left, seg{" " + spinnerFrames[m.spin%len(spinnerFrames)], t.Accent, false})
	}
	msg := m.s.Flash
	if msg == "" {
		msg = keymap.Hints(m.s.Mode())
	}
	left = append(left, seg{" " + msg, t.Hint, false})
	var right []seg
	if v := m.s.UpdateAvailable; v != "" {
		right = append(right, seg{"v" + v + " available · " + m.upgrade + "  ", t.TextDim, false})
	}
	if m.s.MouseOff {
		right = append(right, seg{"mouse off (m)  ", t.Accent, false})
	}
	right = append(right, seg{"? help ", t.Hint, false})
	return []string{
		line(w, t.Strip, seg{strings.Repeat("─", w), t.Border, false}),
		split(w, t.Strip, left, right),
	}
}

func (m *Model) listBody(t Theme, h int) []string {
	l := &m.s.List
	if m.s.Category == app.CatSearch {
		return m.searchBody(t, h)
	}
	if len(l.Items) == 0 {
		switch {
		case l.Err != nil:
			return m.message(t, h, "Stories didn't load", errorText(l.Err), "press r to try again")
		case l.Loading:
			return m.message(t, h, "Fetching stories…", "", "")
		case m.s.Category == app.CatSaved:
			return m.message(t, h, "Nothing saved", "Press s on any story to keep it here.", "")
		case m.s.Category == app.CatHistory:
			return m.message(t, h, "Nothing read yet", "Stories you open are listed here, newest first.", "")
		}
		return m.message(t, h, "This feed is empty", "", "")
	}
	return m.storyRows(t, h)
}

// storyRows renders the list's stories into h lines, scrolled to the cursor.
func (m *Model) storyRows(t Theme, h int) []string {
	l := &m.s.List
	w := m.width
	rows := max(1, h/2)
	if l.Cursor < m.listTop {
		m.listTop = l.Cursor
	}
	if l.Cursor >= m.listTop+rows {
		m.listTop = l.Cursor - rows + 1
	}
	m.listTop = max(0, min(m.listTop, len(l.Items)-rows))

	var out []string
	for i := m.listTop; i < len(l.Items) && i < m.listTop+rows; i++ {
		out = append(out, m.storyRow(t, i, w)...)
	}
	if len(out) < h && l.Loading {
		out = append(out, line(w, t.Body, seg{"      loading more…", t.TextDim, false}))
	}
	return out
}

// searchBody is the Search tab: the query box, how the results were found,
// then the results as an ordinary story list.
func (m *Model) searchBody(t Theme, h int) []string {
	w := m.width
	q := m.s.Search
	l := &m.s.List

	boxBg := t.Strip
	if q.Editing {
		boxBg = t.Highlight
	}
	box := []seg{{" / ", t.Accent, true}, {q.Query, t.Text, true}}
	if q.Editing {
		box = append(box, seg{"▏", t.Accent, false})
	}
	if q.Query == "" {
		box = append(box, seg{"search Hacker News…", t.TextDim, false})
	}
	out := []string{line(w, boxBg, box...), m.searchFilters(t), line(w, t.Body, m.searchInfo(t)...), line(w, t.Body)}
	rest := max(1, h-len(out))

	switch {
	case strings.TrimSpace(q.Query) == "" && !q.Filtered():
		return append(out, m.message(t, rest, "Search every story on Hacker News",
			"Typos are fine. If no story has all your words, the closest ones are shown.",
			"type, then ⏎ or ↓ to browse · change a filter to browse without words")...)
	case len(l.Items) > 0:
		return append(out, m.storyRows(t, rest)...)
	case l.Loading:
		return append(out, m.message(t, rest, "Searching…", "", "")...)
	case l.Err != nil:
		return append(out, m.message(t, rest, "Search didn't work", errorText(l.Err), "r to try again")...)
	}
	return append(out, m.message(t, rest, "Nothing found",
		"Nothing on Hacker News matches, and no story loaded here comes close.", "try other words")...)
}

// searchFilters reads like hn.algolia.com's: "Search Stories by Date for
// Past year", with any filter changed from its default highlighted.
func (m *Model) searchFilters(t Theme) string {
	q, d := m.s.Search, app.DefaultSearch
	val := func(label string, changed bool) seg {
		if changed {
			return seg{label, t.Accent, true}
		}
		return seg{label, t.Text, true}
	}
	left := []seg{
		{" Search ", t.TextDim, false}, val(q.Type.Label(), q.Type != d.Type),
		{" by ", t.TextDim, false}, val(q.Order.Label(), q.Order != d.Order),
		{" for ", t.TextDim, false}, val(q.Range.Label(), q.Range != d.Range),
	}
	right := []seg{{"ctrl+t type · ctrl+o order · ctrl+r range ", t.TextDim, false}}
	return split(m.width, t.Body, left, right)
}

func (m *Model) searchInfo(t Theme) []seg {
	q := m.s.Search
	n := len(m.s.List.Items)
	if n == 0 || m.s.List.Loading && q.Shown == "" {
		return nil
	}
	if q.Shown == "" { // browsing with filters, no words
		return []seg{{" " + plural(n, "result"), t.TextMuted, false}}
	}
	switch q.Kind {
	case hn.SearchAnyWord:
		return []seg{{fmt.Sprintf(" No story has every word of “%s” — %d closest matches", q.Shown, n), t.Accent, false}}
	case hn.SearchLoaded:
		return []seg{{fmt.Sprintf(" Hacker News search found nothing (or is unreachable) — %d fuzzy matches for “%s” among stories loaded here", n, q.Shown), t.Accent, false}}
	}
	return []seg{{fmt.Sprintf(" %s matching “%s”", plural(n, "story"), q.Shown), t.TextMuted, false}}
}

func (m *Model) storyRow(t Theme, i, w int) []string {
	it := m.s.List.Items[i]
	sel := i == m.s.List.Cursor
	visited := m.s.IsViewed(it.ID)
	bg := t.Body
	if sel {
		bg = t.Highlight
	}
	titleFg, rankFg, dim, muted, vote := t.TextBody, t.TextDim, t.TextDim, t.TextMuted, t.Score
	if sel {
		titleFg, rankFg = t.Text, t.Accent
	}
	if visited {
		titleFg, rankFg, dim, muted, vote = t.TextVisited, t.TextVisited, t.TextVisited, t.TextVisited, t.TextVisited
	}
	title := it.Title
	if title == "" {
		title = "(untitled)"
	}
	first := []seg{{fmt.Sprintf(" %3d. ", i+1), rankFg, false}}
	if m.s.IsSaved(it.ID) {
		first = append(first, seg{"★ ", t.Accent, false})
	}
	first = append(first, seg{title, titleFg, sel})
	if host := hostname(it.URL); host != "" {
		first = append(first, seg{" (" + host + ")", dim, false})
	}
	comments := plural(it.Descendants, "comment")
	by := it.By
	if by == "" {
		by = "?"
	}
	second := []seg{
		{"      ", "", false},
		{plural(it.Score, "pt"), vote, false},
		{" · ", dim, false}, {by, muted, false},
		{" · " + relativeTime(it.Time) + " · ", dim, false},
		{comments, muted, false},
	}
	return []string{line(w, bg, first...), line(w, bg, second...)}
}

// detailBody lays the story header and comments out as lines, scrolls so
// the selected comment is in view, and renders only what's visible.
func (m *Model) detailBody(t Theme, h int) []string {
	d := m.s.Detail
	w := m.width
	if m.wrapWidth != w {
		m.wrapWidth, m.wraps = w, map[int][]string{}
	}

	head := m.storyHeader(t, d, w)
	starts := make([]int, len(d.Flat)+1)
	pos := len(head)
	for i, f := range d.Flat {
		starts[i] = pos
		pos += m.commentHeight(f, w)
	}
	starts[len(d.Flat)] = pos
	total := pos

	if len(d.Flat) > 0 {
		c := d.Cursor
		start, end := starts[c], starts[c+1]
		if c == 0 && end <= h {
			m.detailTop = 0 // keep the story header on screen at the top
		} else if start < m.detailTop {
			m.detailTop = start
		} else if end > m.detailTop+h {
			m.detailTop = min(start, end-h)
		}
	} else {
		m.detailTop = 0
	}
	m.detailTop = max(0, min(m.detailTop, total-h))

	out := make([]string, 0, h)
	for i := m.detailTop; i < len(head) && len(out) < h; i++ {
		out = append(out, head[i])
	}
	for i, f := range d.Flat {
		if len(out) >= h {
			break
		}
		if starts[i+1] <= m.detailTop {
			continue
		}
		lines := m.comment(t, f, i == d.Cursor, w)
		skip := max(0, m.detailTop-starts[i])
		for _, ln := range lines[skip:] {
			if len(out) >= h {
				break
			}
			out = append(out, ln)
		}
	}
	return out
}

func (m *Model) storyHeader(t Theme, d *app.Detail, w int) []string {
	s := d.Story
	var out []string
	for _, ln := range m.wrap(-2*s.ID, s.Title, w-2) {
		out = append(out, line(w, t.Body, seg{" " + ln, t.Text, true}))
	}
	if s.URL != "" {
		out = append(out, line(w, t.Body, seg{" " + s.URL, t.Link, false}))
	}
	by := s.By
	if by == "" {
		by = "?"
	}
	saved := ""
	if m.s.IsSaved(s.ID) {
		saved = "  ★ saved"
	}
	out = append(out, line(w, t.Body,
		seg{" " + plural(s.Score, "pt"), t.Score, false},
		seg{" · ", t.TextDim, false}, seg{by, t.TextMuted, false},
		seg{" · " + relativeTime(s.Time) + " · " + plural(s.Descendants, "comment"), t.TextDim, false},
		seg{saved, t.Accent, false},
		seg{sortNote(m.s.Newest), t.Accent, false},
	))
	if s.Text != "" {
		text, _ := hn.RenderHTML(s.Text)
		out = append(out, line(w, t.Body))
		for _, ln := range m.wrap(-2*s.ID-1, text, w-2) {
			out = append(out, line(w, t.Body, seg{" " + ln, t.TextBody, false}))
		}
	}
	out = append(out, line(w, t.Body, seg{" " + strings.Repeat("─", max(0, w-2)), t.Border, false}))

	switch {
	case len(d.Flat) > 0:
	case d.Loading:
		out = append(out, line(w, t.Body, seg{" Fetching the thread…", t.TextDim, false}))
	case d.Err != nil:
		out = append(out, line(w, t.Body, seg{" The thread didn't load (" + errorText(d.Err) + ") — press r to try again", t.TextMuted, false}))
	default:
		out = append(out, line(w, t.Body, seg{" Quiet so far — no comments.", t.TextDim, false}))
	}
	return out
}

const maxIndentDepth = 12

func commentWidth(f app.Flat, w int) (indent, width int) {
	indent = min(f.Depth, maxIndentDepth)*2 + 1
	return indent, max(10, w-indent-3)
}

func (m *Model) commentHeight(f app.Flat, w int) int {
	n := 2 // header + blank line after
	if f.Hidden == 0 && !f.Comment.Deleted {
		_, cw := commentWidth(f, w)
		n += len(m.wrap(f.Comment.ID, f.Comment.Text, cw))
	}
	return n
}

func (m *Model) comment(t Theme, f app.Flat, sel bool, w int) []string {
	c := f.Comment
	indent, cw := commentWidth(f, w)
	pad := strings.Repeat(" ", indent)
	bg := t.Body
	if sel {
		bg = t.Highlight
	}
	bar := t.Depth[f.Depth%len(t.Depth)]
	marker := "│ "
	if sel {
		marker = "▶ "
	}
	head := []seg{{pad, "", false}, {marker, bar, false}}
	if c.Deleted {
		head = append(head, seg{"[deleted]", t.TextDim, false})
	} else {
		head = append(head, seg{c.By, t.Accent, true}, seg{" · " + relativeTime(c.Time), t.TextDim, false})
	}
	if f.ReplyTo != "" {
		head = append(head, seg{" · ↳ " + f.ReplyTo, t.TextMuted, false})
	}
	if f.Hidden > 0 {
		head = append(head, seg{fmt.Sprintf(" · %s folded", plural(f.Hidden, "reply")), t.TextMuted, false})
	} else if n := len(c.Links); n > 0 {
		head = append(head, seg{fmt.Sprintf(" · %s", plural(n, "link")), t.Link, false})
	}
	out := []string{line(w, bg, head...)}
	if f.Hidden == 0 && !c.Deleted {
		for _, ln := range m.wrap(c.ID, c.Text, cw) {
			out = append(out, line(w, bg, seg{pad, "", false}, seg{"│ ", bar, false}, seg{ln, t.TextBody, false}))
		}
	}
	return append(out, line(w, t.Body))
}

func sortNote(newest bool) string {
	if newest {
		return "  · newest first (n)"
	}
	return ""
}

func (m *Model) wrap(key int, text string, width int) []string {
	if lines, ok := m.wraps[key]; ok {
		return lines
	}
	var lines []string
	if text != "" {
		lines = strings.Split(ansi.Wrap(text, width, ""), "\n")
	}
	m.wraps[key] = lines
	return lines
}

func (m *Model) linksBody(t Theme, h int) []string {
	p := m.s.Links
	w := m.width
	out := []string{line(w, t.Body, seg{fmt.Sprintf(" Links (%d)", len(p.Links)), t.Accent, true}), line(w, t.Body)}
	per := 3
	visible := max(1, (h-2)/per)
	top := max(0, p.Cursor-visible+1)
	for i := top; i < len(p.Links) && i < top+visible; i++ {
		l := p.Links[i]
		bg, marker := t.Body, "  "
		if i == p.Cursor {
			bg, marker = t.Highlight, "▶ "
		}
		label := "external"
		if _, ok := hn.ParseItemLink(l.URL); ok {
			label = "opens in app"
		}
		out = append(out,
			line(w, bg, seg{" " + marker, t.Accent, false}, seg{fmt.Sprintf("%d. %s", i+1, l.Text), t.Text, i == p.Cursor}, seg{"  " + label, t.TextDim, false}),
			line(w, bg, seg{"      " + l.URL, t.Link, false}),
			line(w, t.Body))
	}
	return out
}

func (m *Model) helpBody(t Theme, h int) []string {
	w := m.width
	under := app.ModeList
	switch m.s.Screen {
	case app.ScreenDetail:
		under = app.ModeDetail
	case app.ScreenError:
		under = app.ModeError
	}
	out := []string{line(w, t.Body, seg{" Keys", t.Accent, true}), line(w, t.Body)}
	for _, e := range keymap.Help(under) {
		out = append(out, line(w, t.Body, seg{fmt.Sprintf("   %-18s", e.Keys), t.Text, true}, seg{e.Desc, t.TextMuted, false}))
	}
	return append(out, line(w, t.Body), line(w, t.Body, seg{"   esc closes this", t.TextDim, false}))
}

func (m *Model) errorBody(t Theme, h int) []string {
	err := m.s.ResolveErr
	if errors.Is(err, hn.ErrNotFound) {
		return m.message(t, h, "That item is gone", "It was deleted, or the link points at an id HN doesn't have.", "esc to return")
	}
	return m.message(t, h, "That link didn't open", errorText(err), "r to try again · esc to return")
}

func (m *Model) message(t Theme, h int, title, sub, hint string) []string {
	w := m.width
	var lines []string
	add := func(s string, fg lipgloss.Color, bold bool) {
		if s != "" {
			lines = append(lines, centered(w, t.Body, s, fg, bold), line(w, t.Body))
		}
	}
	add(title, t.Accent, true)
	add(sub, t.TextMuted, false)
	add(hint, t.TextDim, false)
	out := make([]string, 0, h)
	for range max(0, (h-len(lines))/2) {
		out = append(out, line(w, t.Body))
	}
	return append(out, lines...)
}

func errorText(err error) string {
	var se *hn.StatusError
	var ne net.Error
	var ue *url.Error
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "The request timed out."
	case errors.Is(err, hn.ErrNotFound):
		return "Not found."
	case errors.As(err, &se):
		return fmt.Sprintf("Hacker News answered HTTP %d.", se.Code)
	case errors.As(err, &ue):
		return "Couldn't reach Hacker News — are you online?"
	}
	return err.Error()
}

func hostname(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

func relativeTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	d := int64(time.Since(time.Unix(unix, 0)).Seconds())
	switch {
	case d < 60:
		return fmt.Sprintf("%ds ago", max(0, d))
	case d < 3600:
		return fmt.Sprintf("%dm ago", d/60)
	case d < 86400:
		return fmt.Sprintf("%dh ago", d/3600)
	case d < 86400*30:
		return fmt.Sprintf("%dd ago", d/86400)
	case d < 86400*365:
		return fmt.Sprintf("%dmo ago", d/2592000)
	}
	return fmt.Sprintf("%dy ago", d/31536000)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	if strings.HasSuffix(word, "y") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(word, "y"))
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// versionLabel is how the header shows the build: "v0.2.0" for a release,
// anything else (a local build) as it is.
func versionLabel(v string) string {
	if v != "" && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}
