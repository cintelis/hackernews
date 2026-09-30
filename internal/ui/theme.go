package ui

import "github.com/charmbracelet/lipgloss"

// Theme colors: the CISO AI brand palette (cisoai-landing's :root tokens).
// The brand is dark-first; the light theme uses the same accents, deepened
// for contrast on a pale background.
type Theme struct {
	Body, Strip, Border       lipgloss.Color
	BrandBg, BrandFg, Brand   lipgloss.Color
	BrandAccent, BrandSubtle  lipgloss.Color
	TabActiveBg, TabActiveFg  lipgloss.Color
	TabFg, Hint               lipgloss.Color
	Text, TextBody, TextMuted lipgloss.Color
	TextDim, TextVisited      lipgloss.Color
	Accent, Score, Link       lipgloss.Color
	Highlight                 lipgloss.Color
	Depth                     []lipgloss.Color
}

// brand tokens
const (
	bg       = "#080b0f"
	bg2      = "#0d1117"
	surface2 = "#182330"
	border   = "#1e2d3d"
	accent   = "#00c8ff"
	accent2  = "#0088bb"
	accent3  = "#00ff88"
	warn     = "#ff6b35"
	text     = "#d7e3ef"
	text2    = "#97a8bc"
	text3    = "#60768d"
	markInk  = "#041219" // text on the hexagon mark
)

var dark = Theme{
	Body: bg, Strip: bg2, Border: border,
	BrandBg: accent, BrandFg: markInk, Brand: text, BrandAccent: accent, BrandSubtle: text3,
	TabActiveBg: accent, TabActiveFg: markInk, TabFg: text2, Hint: text3,
	Text: "#f1f6fb", TextBody: text, TextMuted: text2, TextDim: text3, TextVisited: "#4a5d70",
	Accent: accent, Score: accent3, Link: "#60a5fa", Highlight: surface2,
	Depth: []lipgloss.Color{accent, accent3, "#60a5fa", warn, accent2, text2},
}

var light = Theme{
	Body: "#f4f7fb", Strip: "#e6edf5", Border: "#c9d6e3",
	BrandBg: accent2, BrandFg: "#ffffff", Brand: "#0a1628", BrandAccent: accent2, BrandSubtle: "#52667a",
	TabActiveBg: accent2, TabActiveFg: "#ffffff", TabFg: "#2c3e50", Hint: "#52667a",
	Text: "#0a1628", TextBody: "#1c2b3a", TextMuted: "#3d5266", TextDim: "#5f7387", TextVisited: "#8a9aab",
	Accent: accent2, Score: "#00875a", Link: "#1d4ed8", Highlight: "#ffffff",
	Depth: []lipgloss.Color{accent2, "#00875a", "#1d4ed8", "#c2410c", "#0e7490", "#52667a"},
}
