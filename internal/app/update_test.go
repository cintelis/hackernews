package app

import (
	"errors"
	"testing"
	"time"

	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/store"
)

func newState(t *testing.T) *State {
	t.Helper()
	s := New(nil, nil)
	s.Now = func() time.Time { return time.UnixMilli(1000) }
	return s
}

func find[T any](effs []Effect) (T, bool) {
	for _, e := range effs {
		if v, ok := e.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func must[T any](t *testing.T, effs []Effect) T {
	t.Helper()
	v, ok := find[T](effs)
	if !ok {
		t.Fatalf("expected a %T effect, got %#v", v, effs)
	}
	return v
}

func stories(from, n int) []hn.Item {
	out := make([]hn.Item, n)
	for i := range out {
		out[i] = hn.Item{ID: from + i, Type: "story", Title: "s", Kids: []int{1}}
	}
	return out
}

func ids(from, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = from + i
	}
	return out
}

// loaded returns a state showing the Top feed with n of total stories loaded.
func loaded(t *testing.T, total, n int) *State {
	s := newState(t)
	feed := must[FetchFeed](t, s.Start())
	page := must[FetchItems](t, s.Update(FeedLoaded{Gen: feed.Gen, IDs: ids(1, total)}))
	s.Update(ItemsLoaded{Gen: page.Gen, Items: stories(1, n)})
	return s
}

func TestStartLoadsFirstPage(t *testing.T) {
	s := newState(t)
	feed := must[FetchFeed](t, s.Start())
	if feed.Feed != "top" || !s.Busy() {
		t.Fatalf("feed %+v busy %v", feed, s.Busy())
	}
	page := must[FetchItems](t, s.Update(FeedLoaded{Gen: feed.Gen, IDs: ids(1, 100)}))
	if len(page.IDs) != PageSize || page.IDs[0] != 1 {
		t.Fatalf("first page = %v", page.IDs)
	}
	s.Update(ItemsLoaded{Gen: page.Gen, Items: stories(1, PageSize)})
	if s.Busy() || len(s.List.Items) != PageSize {
		t.Fatalf("items %d busy %v", len(s.List.Items), s.Busy())
	}
}

func TestStaleResultsDropped(t *testing.T) {
	s := newState(t)
	old := must[FetchFeed](t, s.Start())
	s.Update(CmdCategory2) // switch to New before Top answers
	if eff := s.Update(FeedLoaded{Gen: old.Gen, IDs: ids(1, 5)}); len(eff) != 0 || len(s.List.IDs) != 0 {
		t.Fatalf("stale feed applied: %v %v", eff, s.List.IDs)
	}
}

func TestPaginationPrefetchesNearEnd(t *testing.T) {
	s := loaded(t, 100, PageSize)
	for range PageSize - prefetchMargin - 1 {
		if _, ok := find[FetchItems](s.Update(CmdDown)); ok {
			t.Fatalf("fetched next page too early at cursor %d", s.List.Cursor)
		}
	}
	next := must[FetchItems](t, s.Update(CmdDown))
	if next.IDs[0] != PageSize+1 || len(next.IDs) != PageSize {
		t.Fatalf("next page = %v", next.IDs[:3])
	}
	if _, ok := find[FetchItems](s.Update(CmdDown)); ok {
		t.Fatal("requested the same page twice while loading")
	}
}

func TestFeedError(t *testing.T) {
	s := newState(t)
	feed := must[FetchFeed](t, s.Start())
	s.Update(FeedLoaded{Gen: feed.Gen, Err: errors.New("offline")})
	if s.List.Err == nil || s.Busy() {
		t.Fatal("error not recorded")
	}
	retry := must[FetchFeed](t, s.Update(CmdRefresh))
	if !retry.Purge {
		t.Error("refresh should purge the item cache")
	}
}

func TestOpenAndBack(t *testing.T) {
	s := loaded(t, 3, 3)
	s.Update(CmdDown)
	effs := s.Update(CmdOpen)
	thread := must[FetchThread](t, effs)
	must[SaveHistory](t, effs)
	if s.Screen != ScreenDetail || thread.Story.ID != 2 || !s.IsViewed(2) {
		t.Fatalf("screen %v story %d viewed %v", s.Screen, thread.Story.ID, s.IsViewed(2))
	}
	s.Update(ThreadLoaded{Gen: thread.Gen, Tree: []*hn.Comment{{ID: 50}, {ID: 51}}})
	s.Update(CmdBack)
	if s.Screen != ScreenList || s.List.Cursor != 1 {
		t.Fatalf("back: screen %v cursor %d", s.Screen, s.List.Cursor)
	}
}

func tree() []*hn.Comment {
	return []*hn.Comment{
		{ID: 1, Children: []*hn.Comment{{ID: 2, Children: []*hn.Comment{{ID: 3}}}}},
		{ID: 4, Links: []hn.Link{
			{Text: "ext", URL: "https://example.com"},
			{Text: "hn", URL: "https://news.ycombinator.com/item?id=900#901"},
		}},
	}
}

func openDetail(t *testing.T) *State {
	s := loaded(t, 1, 1)
	th := must[FetchThread](t, s.Update(CmdOpen))
	s.Update(ThreadLoaded{Gen: th.Gen, Tree: tree()})
	return s
}

func TestCollapse(t *testing.T) {
	s := openDetail(t)
	if len(s.Detail.Flat) != 4 {
		t.Fatalf("flat = %d", len(s.Detail.Flat))
	}
	s.Update(CmdCollapse) // on comment 1
	if len(s.Detail.Flat) != 2 || s.Detail.Flat[0].Hidden != 2 {
		t.Fatalf("collapsed flat = %+v", s.Detail.Flat)
	}
	s.Update(CmdCollapse)
	if len(s.Detail.Flat) != 4 {
		t.Fatal("expand failed")
	}
}

func TestLinksPopupExternalAndInApp(t *testing.T) {
	s := openDetail(t)
	s.Update(CmdLinks)
	if s.Links != nil || s.Flash == "" {
		t.Fatal("comment without links should flash, not open a popup")
	}
	s.Update(CmdBottom)
	s.Update(CmdLinks)
	if s.Mode() != ModeLinks {
		t.Fatal("popup not open")
	}
	if u := must[OpenURL](t, s.Update(CmdOpen)); u.URL != "https://example.com" {
		t.Errorf("external link opened %q", u.URL)
	}
	s.Update(CmdDown)
	r := must[ResolveLink](t, s.Update(CmdOpen))
	if r.Ref != (hn.ItemRef{ID: 900, Anchor: 901}) || s.Links != nil || !s.Resolving {
		t.Fatalf("in-app link: %+v links %v resolving %v", r.Ref, s.Links, s.Resolving)
	}

	// it resolves: the current thread goes on the stack, focus lands on the anchor
	th := must[FetchThread](t, s.Update(Resolved{Gen: r.Gen, Story: hn.Item{ID: 900, Kids: []int{1}}, Focus: 901}))
	s.Update(ThreadLoaded{Gen: th.Gen, Tree: []*hn.Comment{{ID: 905}, {ID: 901}}})
	if len(s.Stack) != 1 || s.Detail.Story.ID != 900 || s.Detail.Current().ID != 901 {
		t.Fatalf("stack %d story %d cursor on %d", len(s.Stack), s.Detail.Story.ID, s.Detail.Current().ID)
	}
	s.Update(CmdBack)
	if s.Detail.Story.ID != 1 || s.Detail.Current().ID != 4 {
		t.Fatalf("back should restore the previous thread and cursor, got story %d comment %d", s.Detail.Story.ID, s.Detail.Current().ID)
	}
}

func TestResolveCancelAndErrors(t *testing.T) {
	s := openDetail(t)
	s.Update(CmdBottom)
	s.Update(CmdLinks)
	s.Update(CmdDown)
	r := must[ResolveLink](t, s.Update(CmdOpen))

	s.Update(CmdBack) // esc while resolving cancels only the resolve
	if s.Resolving || s.Screen != ScreenDetail {
		t.Fatal("cancel should stay on the thread")
	}
	if eff := s.Update(Resolved{Gen: r.Gen, Story: hn.Item{ID: 900}}); len(eff) != 0 || s.Detail.Story.ID != 1 {
		t.Fatal("cancelled resolve still applied")
	}

	s.Update(CmdLinks)
	s.Update(CmdDown)
	r = must[ResolveLink](t, s.Update(CmdOpen))
	s.Update(Resolved{Gen: r.Gen, Err: errors.New("timeout")})
	if s.Screen != ScreenError || len(s.Stack) != 1 {
		t.Fatalf("screen %v stack %d", s.Screen, len(s.Stack))
	}
	retry := must[ResolveLink](t, s.Update(CmdRefresh))
	s.Update(Resolved{Gen: retry.Gen, Err: hn.ErrNotFound})
	if eff := s.Update(CmdRefresh); len(eff) != 0 {
		t.Fatal("a missing post shouldn't offer retry")
	}
	s.Update(CmdBack)
	if s.Screen != ScreenDetail || s.Detail.Story.ID != 1 {
		t.Fatal("back from error should return to the thread")
	}
}

func TestReturningToStackedThreadStillLoading(t *testing.T) {
	s := openDetail(t)
	s.Update(CmdRefresh) // thread 1 now loading…
	s.Update(CmdBottom)
	s.Detail.Flat = Flatten(tree(), nil) // keep links reachable while "loading"
	s.Update(CmdLinks)
	s.Update(CmdDown)
	r := must[ResolveLink](t, s.Update(CmdOpen))
	s.Update(Resolved{Gen: r.Gen, Story: hn.Item{ID: 900}})
	// …its result would arrive now and be dropped, so going back must refetch
	must[FetchThread](t, s.Update(CmdBack))
}

func TestSaveToggleAndSavedView(t *testing.T) {
	s := loaded(t, 3, 3)
	saved := must[SaveSaved](t, s.Update(CmdToggleSave))
	if len(saved.Entries) != 1 || saved.Entries[0] != (store.Entry{ID: 1, At: 1000}) {
		t.Fatalf("saved = %+v", saved.Entries)
	}
	s.Update(CmdDown)
	s.Update(CmdToggleSave)

	page := must[FetchItems](t, s.Update(CmdSavedView))
	if s.Category != CatSaved || page.IDs[0] != 2 || page.IDs[1] != 1 {
		t.Fatalf("saved view ids = %v (newest first)", page.IDs)
	}
	s.Update(ItemsLoaded{Gen: page.Gen, Items: []hn.Item{{ID: 2}, {ID: 1}}})
	s.Update(CmdToggleSave) // unsave from the saved view
	if len(s.List.Items) != 1 || s.List.Items[0].ID != 1 || s.IsSaved(2) {
		t.Fatalf("unsave in saved view: items %+v", s.List.Items)
	}
}

func TestClearHistoryOnlyInHistoryTab(t *testing.T) {
	s := loaded(t, 2, 2)
	s.Update(CmdOpenHN)
	if len(s.History) != 1 {
		t.Fatal("y should mark viewed")
	}
	if eff := s.Update(CmdClearHistory); len(eff) != 0 || len(s.History) != 1 {
		t.Fatal("x outside history tab must do nothing")
	}
	s.Update(CmdHistoryView)
	must[SaveHistory](t, s.Update(CmdClearHistory))
	if len(s.History) != 0 {
		t.Fatal("history not cleared")
	}
}

func TestHistoryCapAndDedup(t *testing.T) {
	s := newState(t)
	for i := range store.HistoryCap + 10 {
		s.markViewed(i)
	}
	s.markViewed(5)
	if len(s.History) != store.HistoryCap || s.History[0].ID != 5 {
		t.Fatalf("len %d first %d", len(s.History), s.History[0].ID)
	}
	n := 0
	for _, e := range s.History {
		if e.ID == 5 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("id 5 appears %d times", n)
	}
}

func TestOverlaysSwallowKeys(t *testing.T) {
	s := loaded(t, 3, 3)
	s.Update(CmdHelp)
	s.Update(CmdDown)
	if s.List.Cursor != 0 || s.Mode() != ModeHelp {
		t.Fatal("help should swallow navigation")
	}
	s.Update(CmdBack)
	if s.Mode() != ModeList {
		t.Fatal("esc should close help")
	}
	must[Quit](t, s.Update(CmdQuit))
}

func TestCategoryCycling(t *testing.T) {
	s := newState(t)
	s.Start()
	s.Update(CmdPrevCategory)
	if s.Category != CatSaved {
		t.Fatalf("prev from Top = %v", s.Category)
	}
	s.Update(CmdNextCategory)
	if s.Category != CatTop {
		t.Fatalf("next from Saved = %v", s.Category)
	}
	if f := must[FetchFeed](t, s.Update(CmdCategory6)); f.Feed != "job" {
		t.Fatalf("6 = %q", f.Feed)
	}
}
