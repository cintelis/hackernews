package app

import (
	"errors"
	"slices"
	"strings"

	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/store"
)

// Update applies an action and returns the effects it asks for. Results
// carry the generation of the request that produced them; a result whose
// generation is no longer current (the user moved on) is dropped.
func (s *State) Update(a Action) []Effect {
	switch a := a.(type) {
	case Command:
		s.Flash = ""
		return s.command(a)

	case FeedLoaded:
		l := &s.List
		if a.Gen != l.gen {
			return nil
		}
		l.Loading = false
		if a.Err != nil {
			l.Err = a.Err
			return nil
		}
		l.IDs = a.IDs
		return s.requestPage(false)

	case ItemsLoaded:
		l := &s.List
		if a.Gen != l.gen {
			return nil
		}
		l.Loading = false
		l.Items = append(l.Items, a.Items...)
		if len(l.Items) == 0 && len(a.Items) == 0 {
			l.Err = ErrNoStories
		}
		return s.maybeLoadMore()

	case ThreadLoaded:
		d := s.Detail
		if d == nil || a.Gen != d.gen {
			return nil
		}
		d.Loading = false
		if a.Story != nil {
			d.Story = *a.Story
		}
		if a.Err != nil {
			d.Err = a.Err
			return nil
		}
		d.Err = nil
		target := d.focus
		if cur := d.Current(); target == 0 && cur != nil && !s.Newest {
			target = cur.ID // a refresh keeps the cursor on the same comment
		}
		d.focus = 0
		d.Tree = a.Tree
		d.reflatten(s.Newest)
		if i := d.indexOf(target); i >= 0 {
			d.Cursor = i
		} else if s.Newest {
			d.Cursor = 0 // newest first: land on the latest comment
		}
		d.Cursor = clamp(d.Cursor, len(d.Flat))
		return nil

	case Resolved:
		if !s.Resolving || a.Gen != s.resolveGen {
			return nil
		}
		s.Resolving = false
		if a.Err != nil {
			if s.Screen == ScreenDetail && s.Detail != nil {
				s.Stack = append(s.Stack, s.Detail)
				s.Detail = nil
			}
			s.Screen = ScreenError
			s.ResolveErr = a.Err
			return nil
		}
		return s.enterDetail(a.Story, a.Focus)

	case UpdateFound:
		s.UpdateAvailable = a.Version
	case Resized:
		s.ListPage, s.DetailPage = max(1, a.ListPage), max(1, a.DetailPage)
	case Flash:
		s.Flash = a.Text

	case Typed:
		if s.Mode() != ModeSearch {
			return nil
		}
		s.Search.Query += oneLine.Replace(hn.Sanitize(a.Text)) // a paste may carry newlines
		return s.searchChanged()

	case SearchLoaded:
		l := &s.List
		if a.Gen != l.gen {
			return nil
		}
		l.Loading, l.Err, l.Items, l.Cursor = false, a.Err, a.Items, 0
		l.IDs = make([]int, len(a.Items))
		for i, it := range a.Items {
			l.IDs[i] = it.ID
		}
		l.Requested = len(l.IDs) // search results come in one page
		s.Search.Shown, s.Search.Kind = a.Query, a.Kind
	}
	return nil
}

func (s *State) command(c Command) []Effect {
	if c == CmdQuit {
		return []Effect{Quit{}}
	}
	switch s.Mode() {
	case ModeHelp:
		if c == CmdHelp || c == CmdBack {
			s.Help = false
		}
		return nil
	case ModeLinks:
		return s.linksCommand(c)
	case ModeSearch:
		return s.searchCommand(c)
	}

	switch c {
	case CmdHelp:
		s.Help = true
		return nil
	case CmdTheme:
		s.Light = !s.Light
		return nil
	case CmdSavedView:
		return s.goHome(CatSaved)
	case CmdHistoryView:
		return s.goHome(CatHistory)
	case CmdSearch:
		return s.openSearch()
	case CmdBack:
		if s.Resolving { // esc while a link resolves cancels just that
			s.cancelResolve()
			return nil
		}
	}

	switch s.Screen {
	case ScreenDetail:
		return s.detailCommand(c)
	case ScreenError:
		return s.errorCommand(c)
	}
	return s.listCommand(c)
}

func (s *State) listCommand(c Command) []Effect {
	l := &s.List
	var cur *hn.Item
	if l.Cursor < len(l.Items) {
		cur = &l.Items[l.Cursor]
	}
	half := max(1, s.ListPage/2)

	switch c {
	case CmdDown, CmdUp, CmdTop, CmdBottom, CmdHalfDown, CmdHalfUp:
		l.Cursor = move(c, l.Cursor, len(l.Items), half)
		return s.maybeLoadMore()
	case CmdNextCategory, CmdPrevCategory:
		step := 1
		if c == CmdPrevCategory {
			step = -1
		}
		// Search is skipped: landing on it starts typing, which would swallow
		// the next h/l and trap the user there
		n := len(Categories)
		next := s.Category
		for {
			next = Categories[(int(next)+step+n)%n]
			if next != CatSearch {
				break
			}
		}
		return s.switchCategory(next)
	case CmdCategory1, CmdCategory2, CmdCategory3, CmdCategory4, CmdCategory5, CmdCategory6:
		return s.switchCategory(Category(c - CmdCategory1))
	case CmdRefresh:
		return s.loadList(true)
	case CmdBack:
		if s.Category == CatSearch {
			return s.switchCategory(s.prevCategory)
		}
		return nil
	case CmdSearchType, CmdSearchOrder, CmdSearchRange:
		if s.Category == CatSearch {
			return s.cycleSearchFilter(c)
		}
		return nil
	case CmdClearHistory:
		if s.Category != CatHistory {
			return nil
		}
		s.History = nil
		s.reindex()
		return append([]Effect{SaveHistory{}}, s.loadList(false)...)
	}

	if cur == nil {
		return nil
	}
	switch c {
	case CmdOpen:
		return s.enterDetail(*cur, 0)
	case CmdOpenURL:
		url := cur.URL
		if url == "" { // Ask HN and friends have no link: their page is the post
			url = hn.ItemURL(cur.ID)
		}
		return append(s.markViewed(cur.ID), OpenURL{URL: url})
	case CmdOpenHN:
		return append(s.markViewed(cur.ID), OpenURL{URL: hn.ItemURL(cur.ID)})
	case CmdToggleSave:
		return s.toggleSave(cur.ID)
	}
	return nil
}

func (s *State) detailCommand(c Command) []Effect {
	d := s.Detail
	half := max(1, s.DetailPage/2)

	switch c {
	case CmdDown, CmdUp, CmdTop, CmdBottom, CmdHalfDown, CmdHalfUp:
		d.Cursor = move(c, d.Cursor, len(d.Flat), half)
	case CmdBack:
		return s.popView()
	case CmdCollapse:
		if s.Newest {
			s.Flash = "folding works in ranked order — press n to switch"
			return nil
		}
		if cur := d.Current(); cur != nil {
			// the collapsed comment keeps its row, so the cursor index stays valid
			d.Collapsed[cur.ID] = !d.Collapsed[cur.ID]
			d.reflatten(false)
		}
	case CmdSortNewest:
		s.Newest = !s.Newest
		d.resort(s.Newest)
		if s.Newest {
			d.Cursor = 0 // straight to the latest comment
			s.Flash = "newest comments first"
		} else {
			s.Flash = "ranked order — the comment you were on, in its thread"
		}
	case CmdLinks:
		cur := d.Current()
		if cur == nil || len(cur.Links) == 0 {
			s.Flash = "no links in this comment"
			return nil
		}
		s.Links = &LinksPopup{Links: cur.Links}
	case CmdOpenURL:
		url := d.Story.URL
		if url == "" {
			url = hn.ItemURL(d.Story.ID)
		}
		return []Effect{OpenURL{URL: url}}
	case CmdOpenHN:
		return []Effect{OpenURL{URL: hn.ItemURL(d.Story.ID)}}
	case CmdReply:
		// the browser, where the user is logged in: the app never holds HN credentials
		cur := d.Current()
		if cur == nil { // no comments yet: the story's page has the comment box
			s.Flash = "opening the story in your browser to comment"
			return []Effect{OpenURL{URL: hn.ItemURL(d.Story.ID)}}
		}
		if cur.Deleted {
			s.Flash = "can't reply to a deleted comment"
			return nil
		}
		s.Flash = "opening the reply page in your browser"
		return []Effect{OpenURL{URL: hn.ReplyURL(d.Story.ID, cur.ID)}}
	case CmdToggleSave:
		return s.toggleSave(d.Story.ID)
	case CmdRefresh:
		d.gen = s.gen()
		d.Loading = true
		return []Effect{FetchThread{Gen: d.gen, Story: d.Story, Refresh: true}}
	}
	return nil
}

func (s *State) errorCommand(c Command) []Effect {
	switch c {
	case CmdBack:
		return s.popView()
	case CmdRefresh:
		if !errors.Is(s.ResolveErr, hn.ErrNotFound) { // a missing post stays missing
			return s.startResolve(s.ResolveRef)
		}
	}
	return nil
}

func (s *State) linksCommand(c Command) []Effect {
	p := s.Links
	switch c {
	case CmdDown, CmdUp, CmdTop, CmdBottom, CmdHalfDown, CmdHalfUp:
		p.Cursor = move(c, p.Cursor, len(p.Links), 5)
	case CmdBack:
		s.Links = nil
	case CmdOpen:
		link := p.Links[p.Cursor]
		if ref, ok := hn.ParseItemLink(link.URL); ok {
			s.Links = nil
			return s.startResolve(ref)
		}
		return []Effect{OpenURL{URL: link.URL}}
	case CmdOpenURL:
		return []Effect{OpenURL{URL: p.Links[p.Cursor].URL}}
	}
	return nil
}

func (s *State) switchCategory(c Category) []Effect {
	if c == CatSearch && s.Category != CatSearch {
		s.prevCategory = s.Category
		s.Search.Editing = true
	}
	s.Category = c
	return s.loadList(false)
}

func (s *State) loadList(purge bool) []Effect {
	s.List = List{gen: s.gen()}
	switch s.Category {
	case CatSaved:
		s.List.IDs = entryIDs(s.Saved)
	case CatHistory:
		s.List.IDs = entryIDs(s.History)
	case CatSearch:
		s.Search.Shown, s.Search.asked = "", "" // a reload always asks again
		return s.searchChanged()
	default:
		s.List.Loading = true
		return []Effect{FetchFeed{Gen: s.List.gen, Feed: s.Category.Feed(), Purge: purge}}
	}
	return s.requestPage(purge)
}

func (s *State) requestPage(purge bool) []Effect {
	l := &s.List
	if l.Loading || !l.HasMore() {
		return nil
	}
	end := min(l.Requested+PageSize, len(l.IDs))
	ids := slices.Clone(l.IDs[l.Requested:end])
	l.Requested = end
	l.Loading = true
	return []Effect{FetchItems{Gen: l.gen, IDs: ids, Purge: purge}}
}

func (s *State) maybeLoadMore() []Effect {
	if s.List.Cursor >= len(s.List.Items)-prefetchMargin {
		return s.requestPage(false)
	}
	return nil
}

// enterDetail opens a story. From a detail view, the current one is kept on
// the stack so back returns to it exactly as it was left.
func (s *State) enterDetail(story hn.Item, focus int) []Effect {
	effs := s.markViewed(story.ID)
	if s.Screen == ScreenDetail && s.Detail != nil {
		s.Stack = append(s.Stack, s.Detail)
	}
	d := &Detail{Story: story, Collapsed: map[int]bool{}, focus: focus, gen: s.gen()}
	s.Detail, s.Screen, s.ResolveErr = d, ScreenDetail, nil
	if len(story.Kids) > 0 || story.Descendants > 0 {
		d.Loading = true
		effs = append(effs, FetchThread{Gen: d.gen, Story: story})
	}
	return effs
}

// popView goes back one level: the previous thread, or the list.
func (s *State) popView() []Effect {
	s.cancelResolve()
	s.ResolveErr = nil
	if len(s.Stack) == 0 {
		s.Screen, s.Detail = ScreenList, nil
		return nil
	}
	d := s.Stack[len(s.Stack)-1]
	s.Stack = s.Stack[:len(s.Stack)-1]
	s.Detail, s.Screen = d, ScreenDetail
	if d.newest != s.Newest { // the order was switched while it was stacked
		d.resort(s.Newest)
	}
	if d.Loading { // its load finished while it was stacked, and was dropped
		d.gen = s.gen()
		return []Effect{FetchThread{Gen: d.gen, Story: d.Story}}
	}
	return nil
}

func (s *State) goHome(c Category) []Effect {
	s.cancelResolve()
	s.Stack, s.Detail, s.Screen, s.ResolveErr = nil, nil, ScreenList, nil
	return s.switchCategory(c)
}

func (s *State) startResolve(ref hn.ItemRef) []Effect {
	s.Resolving, s.ResolveRef, s.resolveGen = true, ref, s.gen()
	return []Effect{ResolveLink{Gen: s.resolveGen, Ref: ref}}
}

func (s *State) cancelResolve() {
	s.Resolving = false
	s.resolveGen = 0
}

func (s *State) markViewed(id int) []Effect {
	h := slices.DeleteFunc(s.History, func(e store.Entry) bool { return e.ID == id })
	h = append([]store.Entry{{ID: id, At: s.Now().UnixMilli()}}, h...)
	if len(h) > store.HistoryCap {
		h = h[:store.HistoryCap]
	}
	s.History = h
	s.reindex()
	return []Effect{SaveHistory{Entries: slices.Clone(h)}}
}

func (s *State) toggleSave(id int) []Effect {
	if s.savedSet[id] {
		s.Saved = slices.DeleteFunc(s.Saved, func(e store.Entry) bool { return e.ID == id })
		if s.Category == CatSaved && s.Screen == ScreenList {
			s.removeFromList(id)
		}
	} else {
		s.Saved = append([]store.Entry{{ID: id, At: s.Now().UnixMilli()}}, s.Saved...)
	}
	s.reindex()
	return []Effect{SaveSaved{Entries: slices.Clone(s.Saved)}}
}

// removeFromList drops an unsaved story from the Saved list in place.
func (s *State) removeFromList(id int) {
	l := &s.List
	if i := slices.Index(l.IDs, id); i >= 0 {
		l.IDs = slices.Delete(l.IDs, i, i+1)
		if i < l.Requested {
			l.Requested--
		}
	}
	l.Items = slices.DeleteFunc(l.Items, func(it hn.Item) bool { return it.ID == id })
	l.Cursor = clamp(l.Cursor, len(l.Items))
}

func move(c Command, cur, n, half int) int {
	switch c {
	case CmdDown:
		cur++
	case CmdUp:
		cur--
	case CmdTop:
		cur = 0
	case CmdBottom:
		cur = n - 1
	case CmdHalfDown:
		cur += half
	case CmdHalfUp:
		cur -= half
	}
	return clamp(cur, n)
}

func clamp(i, n int) int { return max(0, min(i, n-1)) }

func entryIDs(es []store.Entry) []int {
	ids := make([]int, len(es))
	for i, e := range es {
		ids[i] = e.ID
	}
	return ids
}

var oneLine = strings.NewReplacer("\n", " ", "\r", " ")

// openSearch jumps to the Search tab from anywhere and focuses the box,
// keeping the last query and its results.
func (s *State) openSearch() []Effect {
	s.cancelResolve()
	s.Stack, s.Detail, s.Screen, s.ResolveErr = nil, nil, ScreenList, nil
	s.Search.Editing = true
	if s.Category == CatSearch {
		return nil
	}
	return s.switchCategory(CatSearch)
}

func (s *State) searchCommand(c Command) []Effect {
	switch c {
	case CmdBack:
		s.Search.Editing = false
		if strings.TrimSpace(s.Search.Query) == "" && !s.Search.Filtered() { // nothing searched: leave the tab
			return s.switchCategory(s.prevCategory)
		}
	case CmdSearchType, CmdSearchOrder, CmdSearchRange:
		return s.cycleSearchFilter(c)
	case CmdOpen, CmdDown: // into the results
		s.Search.Editing = false
	case CmdDeleteChar:
		if r := []rune(s.Search.Query); len(r) > 0 {
			s.Search.Query = string(r[:len(r)-1])
			return s.searchChanged()
		}
	case CmdClearInput:
		s.Search.Query = ""
		return s.searchChanged()
	}
	return nil
}

// searchChanged asks for results for the current query. The previous
// results stay listed until the new ones arrive, so typing doesn't flicker.
func (s *State) searchChanged() []Effect {
	q := strings.TrimSpace(s.Search.Query)
	key := s.Search.key(q)
	if key == s.Search.asked {
		return nil // only whitespace changed
	}
	s.Search.asked = key
	s.List.gen = s.gen()
	if q == "" && !s.Search.Filtered() {
		s.List = List{gen: s.List.gen}
		s.Search.Shown = ""
		return nil
	}
	s.List.Loading, s.List.Err = true, nil
	return []Effect{RunSearch{Gen: s.List.gen, Query: q, Options: s.Search.options(s.Now())}}
}
