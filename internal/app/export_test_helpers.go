package app

// Generation accessors for other packages' tests (the UI render tests feed
// results straight into a State without running effects).

func (s *State) ListGenForTest() int { return s.List.gen }

func (s *State) DetailGenForTest() int {
	if s.Detail == nil {
		return 0
	}
	return s.Detail.gen
}
