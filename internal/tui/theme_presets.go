package tui

// ThemeColors defines the color palette for a TUI theme.
type ThemeColors struct {
	Name      string
	Primary   string
	Secondary string
	Accent    string
	Success   string
	Error     string
	Magenta   string
	White     string
	Bright    string
	Dim       string
	Border    string
}

// ThemeNames returns theme names in display order.
var ThemeNames = []string{
	"default",
	"catppuccin",
	"tokyo-night",
	"gruvbox",
	"dracula",
	"nord",
	"rose-pine",
	"kanagawa",
	"solarized",
}

var Themes = map[string]ThemeColors{
	"default": {
		Name:      "Default",
		Primary:   "#818CF8",
		Secondary: "#9CA3AF",
		Accent:    "#FBBF24",
		Success:   "#34D399",
		Error:     "#F87171",
		Magenta:   "#C084FC",
		White:     "#E5E7EB",
		Bright:    "#F9FAFB",
		Dim:       "#4B5563",
		Border:    "#374151",
	},
	"catppuccin": {
		Name:      "Catppuccin Mocha",
		Primary:   "#89B4FA",
		Secondary: "#A6ADC8",
		Accent:    "#F9E2AF",
		Success:   "#A6E3A1",
		Error:     "#F38BA8",
		Magenta:   "#CBA6F7",
		White:     "#CDD6F4",
		Bright:    "#BAC2DE",
		Dim:       "#585B70",
		Border:    "#45475A",
	},
	"tokyo-night": {
		Name:      "Tokyo Night",
		Primary:   "#7AA2F7",
		Secondary: "#A9B1D6",
		Accent:    "#E0AF68",
		Success:   "#9ECE6A",
		Error:     "#F7768E",
		Magenta:   "#BB9AF7",
		White:     "#C0CAF5",
		Bright:    "#C0CAF5",
		Dim:       "#565F89",
		Border:    "#3B4261",
	},
	"gruvbox": {
		Name:      "Gruvbox Dark",
		Primary:   "#83A598",
		Secondary: "#A89984",
		Accent:    "#FABD2F",
		Success:   "#B8BB26",
		Error:     "#FB4934",
		Magenta:   "#D3869B",
		White:     "#EBDBB2",
		Bright:    "#FBF1C7",
		Dim:       "#665C54",
		Border:    "#504945",
	},
	"dracula": {
		Name:      "Dracula",
		Primary:   "#BD93F9",
		Secondary: "#6272A4",
		Accent:    "#F1FA8C",
		Success:   "#50FA7B",
		Error:     "#FF5555",
		Magenta:   "#FF79C6",
		White:     "#F8F8F2",
		Bright:    "#F8F8F2",
		Dim:       "#6272A4",
		Border:    "#44475A",
	},
	"nord": {
		Name:      "Nord",
		Primary:   "#88C0D0",
		Secondary: "#4C566A",
		Accent:    "#EBCB8B",
		Success:   "#A3BE8C",
		Error:     "#BF616A",
		Magenta:   "#B48EAD",
		White:     "#D8DEE9",
		Bright:    "#ECEFF4",
		Dim:       "#434C5E",
		Border:    "#3B4252",
	},
	"rose-pine": {
		Name:      "Rose Pine",
		Primary:   "#C4A7E7",
		Secondary: "#6E6A86",
		Accent:    "#F6C177",
		Success:   "#9CCFD8",
		Error:     "#EB6F92",
		Magenta:   "#EBBCBA",
		White:     "#E0DEF4",
		Bright:    "#E0DEF4",
		Dim:       "#524F67",
		Border:    "#403D52",
	},
	"kanagawa": {
		Name:      "Kanagawa",
		Primary:   "#7E9CD8",
		Secondary: "#727169",
		Accent:    "#E6C384",
		Success:   "#98BB6C",
		Error:     "#E46876",
		Magenta:   "#957FB8",
		White:     "#DCD7BA",
		Bright:    "#DCD7BA",
		Dim:       "#54546D",
		Border:    "#363646",
	},
	"solarized": {
		Name:      "Solarized Dark",
		Primary:   "#268BD2",
		Secondary: "#586E75",
		Accent:    "#B58900",
		Success:   "#859900",
		Error:     "#DC322F",
		Magenta:   "#D33682",
		White:     "#839496",
		Bright:    "#93A1A1",
		Dim:       "#586E75",
		Border:    "#073642",
	},
}
