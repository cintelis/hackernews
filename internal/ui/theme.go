package ui

import "github.com/charmbracelet/lipgloss"

// Theme colors. The orange and the light theme's beige are Hacker News's own.
type Theme struct {
	Body, Strip, Border       lipgloss.Color
	BrandBg, BrandFg, Brand   lipgloss.Color
	BrandSubtle               lipgloss.Color
	TabActiveBg, TabActiveFg  lipgloss.Color
	TabFg, Hint               lipgloss.Color
	Text, TextBody, TextMuted lipgloss.Color
	TextDim, TextVisited      lipgloss.Color
	Accent, Link, Highlight   lipgloss.Color
	Depth                     []lipgloss.Color
}

var dark = Theme{
	Body: "", Strip: "#121212", Border: "#333333",
	BrandBg: "#ff6600", BrandFg: "#111111", Brand: "#ff7a1a", BrandSubtle: "#8c8c8c",
	TabActiveBg: "#ff6600", TabActiveFg: "#111111", TabFg: "#bdbdbd", Hint: "#8c8c8c",
	Text: "#f2f2f2", TextBody: "#d0d0d0", TextMuted: "#a0a0a0", TextDim: "#707070", TextVisited: "#5a5a5a",
	Accent: "#ff6600", Link: "#6aa9ff", Highlight: "#242424",
	Depth: []lipgloss.Color{"#ff6600", "#e8b04b", "#6cc98a", "#5fb0e5", "#b38ad6", "#e9788a"},
}

var light = Theme{
	Body: "#f6f6ef", Strip: "#ff6600", Border: "#e05a00",
	BrandBg: "#ffffff", BrandFg: "#111111", Brand: "#111111", BrandSubtle: "#4a1d00",
	TabActiveBg: "#ffffff", TabActiveFg: "#111111", TabFg: "#1a1a1a", Hint: "#1a1a1a",
	Text: "#111111", TextBody: "#1f1f1f", TextMuted: "#444444", TextDim: "#6b6b6b", TextVisited: "#8f8f8f",
	Accent: "#e65c00", Link: "#1a5fb4", Highlight: "#ffffff",
	Depth: []lipgloss.Color{"#e65c00", "#a86b00", "#2e7d4f", "#1f6fa8", "#744aa3", "#a8384d"},
}
