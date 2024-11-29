package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	// Podstawowe kolory
	Subtle    lipgloss.Color
	Highlight lipgloss.Color
	Special   lipgloss.Color
	Error     lipgloss.Color
	StatusBar lipgloss.Color
	Border    lipgloss.Color

	// Kolory elementów menu i informacji
	ItemColor     lipgloss.Color
	InfotextColor lipgloss.Color
	HostColor     lipgloss.Color
	LabelColor    lipgloss.Color
	InputColor    lipgloss.Color

	// Kolory dla typów plików
	DirectoryColor    lipgloss.Color
	ExecutableColor   lipgloss.Color
	ArchiveColor      lipgloss.Color
	ImageColor        lipgloss.Color
	DocumentColor     lipgloss.Color
	DefaultFileColor  lipgloss.Color
	SelectedFileColor lipgloss.Color

	// Kolory dla plików kodu
	CodeCColor       lipgloss.Color
	CodeHColor       lipgloss.Color
	CodeGoColor      lipgloss.Color
	CodePyColor      lipgloss.Color
	CodeJsColor      lipgloss.Color
	CodeJsonColor    lipgloss.Color
	CodeDefaultColor lipgloss.Color
}

var (
	currentThemeIndex = 0

	themes = []Theme{
		{
			// Domyślny motyw (obecny)
			Subtle:    lipgloss.Color("#6C7086"),
			Highlight: lipgloss.Color("#7DC4E4"),
			Special:   lipgloss.Color("#FF9E64"),
			Error:     lipgloss.Color("#F38BA8"),
			StatusBar: lipgloss.Color("#E7E7E7"),
			Border:    lipgloss.Color("#33B2FF"),

			ItemColor:     lipgloss.Color("#FF3A99"),
			InfotextColor: lipgloss.Color("#FF3A99"),
			HostColor:     lipgloss.Color("#2DAFFF"),
			LabelColor:    lipgloss.Color("#A6ADC8"),
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#1E90FF"),
			ExecutableColor:   lipgloss.Color("#32CD32"),
			ArchiveColor:      lipgloss.Color("#BA55D3"),
			ImageColor:        lipgloss.Color("#FF8C00"),
			DocumentColor:     lipgloss.Color("#FFD700"),
			DefaultFileColor:  lipgloss.Color("#A9A9A9"),
			SelectedFileColor: lipgloss.Color("#FF1493"),

			CodeCColor:       lipgloss.Color("#00CED1"),
			CodeHColor:       lipgloss.Color("#4682B4"),
			CodeGoColor:      lipgloss.Color("#2E8B57"),
			CodePyColor:      lipgloss.Color("#6A5ACD"),
			CodeJsColor:      lipgloss.Color("#DAA520"),
			CodeJsonColor:    lipgloss.Color("#7FFF00"),
			CodeDefaultColor: lipgloss.Color("#708090"),
		},
		{
			// Dracula Classic - motyw inspirowany klasycznym schematem kolorów Dracula
			Subtle:    lipgloss.Color("#6272A4"), // Delikatny fioletowy
			Highlight: lipgloss.Color("#8BE9FD"), // Jasny cyan
			Special:   lipgloss.Color("#FF79C6"), // Różowy
			Error:     lipgloss.Color("#FF5555"), // Czerwony
			StatusBar: lipgloss.Color("#44475A"), // Ciemny szary z fioletowym odcieniem
			Border:    lipgloss.Color("#BD93F9"), // Jasny fioletowy

			ItemColor:     lipgloss.Color("#50FA7B"), // Zielony
			InfotextColor: lipgloss.Color("#F1FA8C"), // Żółty
			HostColor:     lipgloss.Color("#8BE9FD"), // Jasny cyan
			LabelColor:    lipgloss.Color("#F8F8F2"), // Jasny szary
			InputColor:    lipgloss.Color("#F8F8F2"), // Jasny szary

			DirectoryColor:    lipgloss.Color("#BD93F9"), // Jasny fioletowy
			ExecutableColor:   lipgloss.Color("#50FA7B"), // Zielony
			ArchiveColor:      lipgloss.Color("#FFB86C"), // Pomarańczowy
			ImageColor:        lipgloss.Color("#FF79C6"), // Różowy
			DocumentColor:     lipgloss.Color("#F1FA8C"), // Żółty
			DefaultFileColor:  lipgloss.Color("#F8F8F2"), // Jasny szary
			SelectedFileColor: lipgloss.Color("#6272A4"), // Delikatny fioletowy

			CodeCColor:       lipgloss.Color("#8BE9FD"), // Jasny cyan
			CodeHColor:       lipgloss.Color("#BD93F9"), // Jasny fioletowy
			CodeGoColor:      lipgloss.Color("#50FA7B"), // Zielony
			CodePyColor:      lipgloss.Color("#FF79C6"), // Różowy
			CodeJsColor:      lipgloss.Color("#F1FA8C"), // Żółty
			CodeJsonColor:    lipgloss.Color("#FFB86C"), // Pomarańczowy
			CodeDefaultColor: lipgloss.Color("#F8F8F2"), // Jasny szary
		},
		{
			// Dracula Night - motyw z ciemniejszymi odcieniami inspirowanymi Dracula Theme
			Subtle:    lipgloss.Color("#44475A"), // Ciemny szary z fioletowym odcieniem
			Highlight: lipgloss.Color("#BD93F9"), // Jasny fioletowy
			Special:   lipgloss.Color("#FFB86C"), // Pomarańczowy
			Error:     lipgloss.Color("#FF5555"), // Czerwony
			StatusBar: lipgloss.Color("#282A36"), // Bardzo ciemny szary
			Border:    lipgloss.Color("#50FA7B"), // Zielony

			ItemColor:     lipgloss.Color("#FF79C6"), // Różowy
			InfotextColor: lipgloss.Color("#8BE9FD"), // Jasny cyan
			HostColor:     lipgloss.Color("#BD93F9"), // Jasny fioletowy
			LabelColor:    lipgloss.Color("#F8F8F2"), // Jasny szary
			InputColor:    lipgloss.Color("#F8F8F2"), // Jasny szary

			DirectoryColor:    lipgloss.Color("#50FA7B"), // Zielony
			ExecutableColor:   lipgloss.Color("#FFB86C"), // Pomarańczowy
			ArchiveColor:      lipgloss.Color("#FF79C6"), // Różowy
			ImageColor:        lipgloss.Color("#8BE9FD"), // Jasny cyan
			DocumentColor:     lipgloss.Color("#F1FA8C"), // Żółty
			DefaultFileColor:  lipgloss.Color("#F8F8F2"), // Jasny szary
			SelectedFileColor: lipgloss.Color("#44475A"), // Ciemny szary z fioletowym odcieniem

			CodeCColor:       lipgloss.Color("#BD93F9"), // Jasny fioletowy
			CodeHColor:       lipgloss.Color("#50FA7B"), // Zielony
			CodeGoColor:      lipgloss.Color("#FFB86C"), // Pomarańczowy
			CodePyColor:      lipgloss.Color("#FF79C6"), // Różowy
			CodeJsColor:      lipgloss.Color("#F1FA8C"), // Żółty
			CodeJsonColor:    lipgloss.Color("#8BE9FD"), // Jasny cyan
			CodeDefaultColor: lipgloss.Color("#F8F8F2"), // Jasny szary
		},
		{
			// VSCodeDark - inspirowany domyślnym motywem VS Code Dark+
			Subtle:    lipgloss.Color("#808080"), // Szary z VS Code
			Highlight: lipgloss.Color("#569CD6"), // Niebieski VS Code
			Special:   lipgloss.Color("#4EC9B0"), // Turkusowy VS Code
			Error:     lipgloss.Color("#F44747"), // Czerwony VS Code
			StatusBar: lipgloss.Color("#007ACC"), // Niebieski statusbar VS Code
			Border:    lipgloss.Color("#569CD6"), // Niebieski VS Code

			ItemColor:     lipgloss.Color("#CE9178"), // Pomarańczowy VS Code
			InfotextColor: lipgloss.Color("#9CDCFE"), // Jasnoniebieski VS Code
			HostColor:     lipgloss.Color("#569CD6"), // Niebieski VS Code
			LabelColor:    lipgloss.Color("#D4D4D4"), // Podstawowy tekst VS Code
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#569CD6"), // Niebieski
			ExecutableColor:   lipgloss.Color("#4EC9B0"), // Turkusowy
			ArchiveColor:      lipgloss.Color("#CE9178"), // Pomarańczowy
			ImageColor:        lipgloss.Color("#C586C0"), // Różowy
			DocumentColor:     lipgloss.Color("#DCDCAA"), // Żółtawy
			DefaultFileColor:  lipgloss.Color("#D4D4D4"), // Podstawowy tekst
			SelectedFileColor: lipgloss.Color("#9CDCFE"), // Jasnoniebieski

			CodeCColor:       lipgloss.Color("#4EC9B0"), // Turkusowy
			CodeHColor:       lipgloss.Color("#569CD6"), // Niebieski
			CodeGoColor:      lipgloss.Color("#4EC9B0"), // Turkusowy
			CodePyColor:      lipgloss.Color("#C586C0"), // Różowy
			CodeJsColor:      lipgloss.Color("#DCDCAA"), // Żółtawy
			CodeJsonColor:    lipgloss.Color("#CE9178"), // Pomarańczowy
			CodeDefaultColor: lipgloss.Color("#D4D4D4"), // Podstawowy tekst
		},
		{
			// DraculaClassic - dokładnie bazujący na palecie Dracula
			Subtle:    lipgloss.Color("#6272a4"), // Comment
			Highlight: lipgloss.Color("#8be9fd"), // Cyan
			Special:   lipgloss.Color("#50fa7b"), // Green
			Error:     lipgloss.Color("#ff5555"), // Red
			StatusBar: lipgloss.Color("#282a36"), // Background
			Border:    lipgloss.Color("#bd93f9"), // Purple

			ItemColor:     lipgloss.Color("#ff79c6"), // Pink
			InfotextColor: lipgloss.Color("#8be9fd"), // Cyan
			HostColor:     lipgloss.Color("#bd93f9"), // Purple
			LabelColor:    lipgloss.Color("#f8f8f2"), // Foreground
			InputColor:    lipgloss.Color("#f8f8f2"), // Foreground

			DirectoryColor:    lipgloss.Color("#8be9fd"), // Cyan
			ExecutableColor:   lipgloss.Color("#50fa7b"), // Green
			ArchiveColor:      lipgloss.Color("#ffb86c"), // Orange
			ImageColor:        lipgloss.Color("#ff79c6"), // Pink
			DocumentColor:     lipgloss.Color("#f1fa8c"), // Yellow
			DefaultFileColor:  lipgloss.Color("#f8f8f2"), // Foreground
			SelectedFileColor: lipgloss.Color("#bd93f9"), // Purple

			CodeCColor:       lipgloss.Color("#50fa7b"), // Green
			CodeHColor:       lipgloss.Color("#8be9fd"), // Cyan
			CodeGoColor:      lipgloss.Color("#50fa7b"), // Green
			CodePyColor:      lipgloss.Color("#ff79c6"), // Pink
			CodeJsColor:      lipgloss.Color("#ffb86c"), // Orange
			CodeJsonColor:    lipgloss.Color("#f1fa8c"), // Yellow
			CodeDefaultColor: lipgloss.Color("#f8f8f2"), // Foreground
		},
		{
			// Molokai - inspirowany klasyczną paletą Molokai
			Subtle:    lipgloss.Color("#808080"), // Szary
			Highlight: lipgloss.Color("#66D9EF"), // Jasnoniebieski molokai
			Special:   lipgloss.Color("#A6E22E"), // Limonkowy molokai
			Error:     lipgloss.Color("#F92672"), // Różowo-czerwony molokai
			StatusBar: lipgloss.Color("#272822"), // Tło molokai
			Border:    lipgloss.Color("#66D9EF"), // Jasnoniebieski molokai

			ItemColor:     lipgloss.Color("#F92672"), // Różowo-czerwony
			InfotextColor: lipgloss.Color("#66D9EF"), // Jasnoniebieski
			HostColor:     lipgloss.Color("#FD971F"), // Pomarańczowy molokai
			LabelColor:    lipgloss.Color("#F8F8F2"), // Jasny tekst molokai
			InputColor:    lipgloss.Color("#F8F8F2"), // Jasny tekst

			DirectoryColor:    lipgloss.Color("#66D9EF"), // Jasnoniebieski
			ExecutableColor:   lipgloss.Color("#A6E22E"), // Limonkowy
			ArchiveColor:      lipgloss.Color("#FD971F"), // Pomarańczowy
			ImageColor:        lipgloss.Color("#AE81FF"), // Fioletowy molokai
			DocumentColor:     lipgloss.Color("#E6DB74"), // Żółty molokai
			DefaultFileColor:  lipgloss.Color("#F8F8F2"), // Jasny tekst
			SelectedFileColor: lipgloss.Color("#F92672"), // Różowo-czerwony

			CodeCColor:       lipgloss.Color("#A6E22E"), // Limonkowy
			CodeHColor:       lipgloss.Color("#66D9EF"), // Jasnoniebieski
			CodeGoColor:      lipgloss.Color("#A6E22E"), // Limonkowy
			CodePyColor:      lipgloss.Color("#AE81FF"), // Fioletowy
			CodeJsColor:      lipgloss.Color("#FD971F"), // Pomarańczowy
			CodeJsonColor:    lipgloss.Color("#E6DB74"), // Żółty
			CodeDefaultColor: lipgloss.Color("#F8F8F2"), // Jasny tekst
		},
		{
			// CyberNeon - inspirowany cyberpunkiem z neonowymi akcentami
			Subtle:    lipgloss.Color("#8B9BB4"), // Jaśniejszy niebieskoszary
			Highlight: lipgloss.Color("#FF2A6D"), // Neonowy różowy
			Special:   lipgloss.Color("#05FFA1"), // Jaskrawy cybernetyczny zielony
			Error:     lipgloss.Color("#FF3366"), // Intensywny czerwony
			StatusBar: lipgloss.Color("#2D3246"), // Ciemny niebieskoszary
			Border:    lipgloss.Color("#FF2A6D"), // Neonowy różowy

			ItemColor:     lipgloss.Color("#FF2A6D"), // Neonowy różowy
			InfotextColor: lipgloss.Color("#05FFA1"), // Cybernetyczny zielony
			HostColor:     lipgloss.Color("#00F1F1"), // Jasny cyjan
			LabelColor:    lipgloss.Color("#C8D3F5"), // Jasny niebieski
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#00F1F1"), // Jasny cyjan
			ExecutableColor:   lipgloss.Color("#05FFA1"), // Cybernetyczny zielony
			ArchiveColor:      lipgloss.Color("#FF9E64"), // Neonowy pomarańczowy
			ImageColor:        lipgloss.Color("#FF2A6D"), // Neonowy różowy
			DocumentColor:     lipgloss.Color("#C792EA"), // Jasny fiolet
			DefaultFileColor:  lipgloss.Color("#8B9BB4"), // Jaśniejszy niebieskoszary
			SelectedFileColor: lipgloss.Color("#00F1F1"), // Jasny cyjan

			CodeCColor:       lipgloss.Color("#05FFA1"),
			CodeHColor:       lipgloss.Color("#00F1F1"),
			CodeGoColor:      lipgloss.Color("#1AEBFF"),
			CodePyColor:      lipgloss.Color("#C792EA"),
			CodeJsColor:      lipgloss.Color("#FFB86C"),
			CodeJsonColor:    lipgloss.Color("#FF9E64"),
			CodeDefaultColor: lipgloss.Color("#8B9BB4"),
		},
		{
			// AtomicDark - inspirowany edytorem Atom i nowoczesnym UI
			Subtle:    lipgloss.Color("#ABB2BF"), // Jasny szary
			Highlight: lipgloss.Color("#61AFEF"), // Jasny niebieski
			Special:   lipgloss.Color("#98C379"), // Zielony atom
			Error:     lipgloss.Color("#E06C75"), // Czerwony atom
			StatusBar: lipgloss.Color("#282C34"), // Ciemny tło
			Border:    lipgloss.Color("#61AFEF"), // Jasny niebieski

			ItemColor:     lipgloss.Color("#C678DD"), // Fioletowy
			InfotextColor: lipgloss.Color("#56B6C2"), // Cyjan
			HostColor:     lipgloss.Color("#61AFEF"), // Jasny niebieski
			LabelColor:    lipgloss.Color("#E5E5E5"), // Bardzo jasny szary
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#61AFEF"), // Jasny niebieski
			ExecutableColor:   lipgloss.Color("#98C379"), // Zielony
			ArchiveColor:      lipgloss.Color("#D19A66"), // Pomarańczowy
			ImageColor:        lipgloss.Color("#C678DD"), // Fioletowy
			DocumentColor:     lipgloss.Color("#56B6C2"), // Cyjan
			DefaultFileColor:  lipgloss.Color("#ABB2BF"), // Jasny szary
			SelectedFileColor: lipgloss.Color("#E5C07B"), // Żółty

			CodeCColor:       lipgloss.Color("#98C379"),
			CodeHColor:       lipgloss.Color("#61AFEF"),
			CodeGoColor:      lipgloss.Color("#56B6C2"),
			CodePyColor:      lipgloss.Color("#C678DD"),
			CodeJsColor:      lipgloss.Color("#E5C07B"),
			CodeJsonColor:    lipgloss.Color("#D19A66"),
			CodeDefaultColor: lipgloss.Color("#ABB2BF"),
		},
		{
			// DraculaPro - inspirowany popularnym motywem Dracula
			Subtle:    lipgloss.Color("#BFBFBF"), // Jasny szary
			Highlight: lipgloss.Color("#BD93F9"), // Fioletowy dracula
			Special:   lipgloss.Color("#50FA7B"), // Zielony dracula
			Error:     lipgloss.Color("#FF5555"), // Czerwony dracula
			StatusBar: lipgloss.Color("#282A36"), // Tło dracula
			Border:    lipgloss.Color("#BD93F9"), // Fioletowy dracula

			ItemColor:     lipgloss.Color("#FF79C6"), // Różowy dracula
			InfotextColor: lipgloss.Color("#8BE9FD"), // Cyjan dracula
			HostColor:     lipgloss.Color("#BD93F9"), // Fioletowy dracula
			LabelColor:    lipgloss.Color("#F8F8F2"), // Biały dracula
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#8BE9FD"), // Cyjan
			ExecutableColor:   lipgloss.Color("#50FA7B"), // Zielony
			ArchiveColor:      lipgloss.Color("#FFB86C"), // Pomarańczowy
			ImageColor:        lipgloss.Color("#FF79C6"), // Różowy
			DocumentColor:     lipgloss.Color("#BD93F9"), // Fioletowy
			DefaultFileColor:  lipgloss.Color("#BFBFBF"), // Jasny szary
			SelectedFileColor: lipgloss.Color("#F1FA8C"), // Żółty

			CodeCColor:       lipgloss.Color("#50FA7B"),
			CodeHColor:       lipgloss.Color("#8BE9FD"),
			CodeGoColor:      lipgloss.Color("#BD93F9"),
			CodePyColor:      lipgloss.Color("#FF79C6"),
			CodeJsColor:      lipgloss.Color("#FFB86C"),
			CodeJsonColor:    lipgloss.Color("#F1FA8C"),
			CodeDefaultColor: lipgloss.Color("#BFBFBF"),
		},
		{
			// Ciemny motyw
			Subtle:    lipgloss.Color("#515671"),
			Highlight: lipgloss.Color("#89DCEB"),
			Special:   lipgloss.Color("#FAB387"),
			Error:     lipgloss.Color("#F38BA8"),
			StatusBar: lipgloss.Color("#C6C9CE"),
			Border:    lipgloss.Color("#89B4FA"),

			ItemColor:     lipgloss.Color("#FF85B5"),
			InfotextColor: lipgloss.Color("#FF85B5"),
			HostColor:     lipgloss.Color("#74C7EC"),
			LabelColor:    lipgloss.Color("#BAC2DE"),
			InputColor:    lipgloss.Color("#CDD6F4"),

			DirectoryColor:    lipgloss.Color("#89B4FA"),
			ExecutableColor:   lipgloss.Color("#A6E3A1"),
			ArchiveColor:      lipgloss.Color("#CBA6F7"),
			ImageColor:        lipgloss.Color("#FAB387"),
			DocumentColor:     lipgloss.Color("#F9E2AF"),
			DefaultFileColor:  lipgloss.Color("#A6ADC8"),
			SelectedFileColor: lipgloss.Color("#F5C2E7"),

			CodeCColor:       lipgloss.Color("#94E2D5"),
			CodeHColor:       lipgloss.Color("#89B4FA"),
			CodeGoColor:      lipgloss.Color("#A6E3A1"),
			CodePyColor:      lipgloss.Color("#CBA6F7"),
			CodeJsColor:      lipgloss.Color("#F9E2AF"),
			CodeJsonColor:    lipgloss.Color("#A6E3A1"),
			CodeDefaultColor: lipgloss.Color("#9399B2"),
		},
		{
			// Aurora - motyw inspirowany zorzą polarną
			Subtle:    lipgloss.Color("#6272A4"), // Delikatny szaro-niebieski
			Highlight: lipgloss.Color("#61AFEF"), // Jasny niebieski
			Special:   lipgloss.Color("#FF79C6"), // Intensywny różowy
			Error:     lipgloss.Color("#FF5555"), // Jasny czerwony
			StatusBar: lipgloss.Color("#282C34"), // Ciemny szaro-niebieski
			Border:    lipgloss.Color("#61AFEF"), // Nowoczesny niebieski

			ItemColor:     lipgloss.Color("#FF79C6"), // Intensywny różowy
			InfotextColor: lipgloss.Color("#FF79C6"), // Intensywny różowy
			HostColor:     lipgloss.Color("#61AFEF"), // Jasny niebieski
			LabelColor:    lipgloss.Color("#FFFFFF"), // Biały tekst
			InputColor:    lipgloss.Color("#FFFFFF"), // Biały tekst

			DirectoryColor:    lipgloss.Color("#61AFEF"), // Jasny niebieski
			ExecutableColor:   lipgloss.Color("#98C379"), // Zielony
			ArchiveColor:      lipgloss.Color("#FFA500"), // Jasny pomarańczowy
			ImageColor:        lipgloss.Color("#61AFEF"), // Jasny niebieski
			DocumentColor:     lipgloss.Color("#98C379"), // Zielony
			DefaultFileColor:  lipgloss.Color("#FFFFFF"), // Biały tekst
			SelectedFileColor: lipgloss.Color("#FF79C6"), // Intensywny różowy

			CodeCColor:       lipgloss.Color("#61AFEF"), // Jasny niebieski
			CodeHColor:       lipgloss.Color("#FF79C6"), // Intensywny różowy
			CodeGoColor:      lipgloss.Color("#98C379"), // Zielony
			CodePyColor:      lipgloss.Color("#FF79C6"), // Intensywny różowy
			CodeJsColor:      lipgloss.Color("#61AFEF"), // Jasny niebieski
			CodeJsonColor:    lipgloss.Color("#61AFEF"), // Jasny niebieski
			CodeDefaultColor: lipgloss.Color("#FFFFFF"), // Biały tekst
		},
		{
			// Cyberpunk - motyw inspirowany futurystycznymi neonami
			Subtle:    lipgloss.Color("#2F2B6D"),
			Highlight: lipgloss.Color("#FF00FF"), // Neonowy fiolet
			Special:   lipgloss.Color("#00FFFF"), // Cyan
			Error:     lipgloss.Color("#FF1493"), // Deep Pink
			StatusBar: lipgloss.Color("#3A3A5A"), // Jasniejszy ciemny fiolet dla lepszej widoczności
			Border:    lipgloss.Color("#FF00FF"),

			ItemColor:     lipgloss.Color("#FF00FF"),
			InfotextColor: lipgloss.Color("#FF00FF"),
			HostColor:     lipgloss.Color("#00FFFF"),
			LabelColor:    lipgloss.Color("#FFFFFF"), // Biały tekst dla lepszej czytelności
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#00FFFF"),
			ExecutableColor:   lipgloss.Color("#39FF14"), // Neonowy zielony
			ArchiveColor:      lipgloss.Color("#FF4500"), // Neonowy pomarańcz
			ImageColor:        lipgloss.Color("#00FFFF"),
			DocumentColor:     lipgloss.Color("#39FF14"),
			DefaultFileColor:  lipgloss.Color("#FFFFFF"),
			SelectedFileColor: lipgloss.Color("#FF00FF"),

			CodeCColor:       lipgloss.Color("#FF00FF"),
			CodeHColor:       lipgloss.Color("#00FFFF"),
			CodeGoColor:      lipgloss.Color("#39FF14"),
			CodePyColor:      lipgloss.Color("#FF00FF"),
			CodeJsColor:      lipgloss.Color("#00FFFF"),
			CodeJsonColor:    lipgloss.Color("#00FFFF"),
			CodeDefaultColor: lipgloss.Color("#FFFFFF"),
		},
		{
			// NeonGreen - motyw z dominującym intensywnym zielonym
			Subtle:    lipgloss.Color("#CCCCCC"), // Jaśniejszy dla lepszej widoczności
			Highlight: lipgloss.Color("#39FF14"), // Neonowy zielony
			Special:   lipgloss.Color("#00FF7F"), // Spring Green
			Error:     lipgloss.Color("#FF4500"), // Neonowy pomarańcz
			StatusBar: lipgloss.Color("#4D4D4D"),
			Border:    lipgloss.Color("#39FF14"),

			ItemColor:     lipgloss.Color("#39FF14"),
			InfotextColor: lipgloss.Color("#39FF14"),
			HostColor:     lipgloss.Color("#00FF7F"),
			LabelColor:    lipgloss.Color("#E0E0E0"), // Jaśniejszy dla lepszej czytelności
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#00FF7F"),
			ExecutableColor:   lipgloss.Color("#7CFC00"),
			ArchiveColor:      lipgloss.Color("#FFD700"),
			ImageColor:        lipgloss.Color("#00FFFF"), // Zmienione na cyan dla lepszego wyróżnienia
			DocumentColor:     lipgloss.Color("#7CFC00"),
			DefaultFileColor:  lipgloss.Color("#E0E0E0"), // Jaśniejszy
			SelectedFileColor: lipgloss.Color("#39FF14"),

			CodeCColor:       lipgloss.Color("#39FF14"),
			CodeHColor:       lipgloss.Color("#00FF7F"),
			CodeGoColor:      lipgloss.Color("#7CFC00"),
			CodePyColor:      lipgloss.Color("#39FF14"),
			CodeJsColor:      lipgloss.Color("#FFD700"), // Zmienione na złoty dla lepszego kontrastu
			CodeJsonColor:    lipgloss.Color("#00FF7F"),
			CodeDefaultColor: lipgloss.Color("#E0E0E0"), // Jaśniejszy
		},
		{
			// RetroOrange - motyw z ciepłym, retro pomarańczowym akcentem
			Subtle:    lipgloss.Color("#D0D0D0"), // Znacznie jaśniejszy dla lepszej widoczności
			Highlight: lipgloss.Color("#FFA500"), // Pomarańczowy
			Special:   lipgloss.Color("#FF8C00"), // Dark Orange
			Error:     lipgloss.Color("#DC143C"), // Crimson
			StatusBar: lipgloss.Color("#444444"),
			Border:    lipgloss.Color("#FFA500"),

			ItemColor:     lipgloss.Color("#FFA500"),
			InfotextColor: lipgloss.Color("#FFA500"),
			HostColor:     lipgloss.Color("#FF8C00"),
			LabelColor:    lipgloss.Color("#E8E8E8"), // Jaśniejszy dla lepszej czytelności
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#FF8C00"),
			ExecutableColor:   lipgloss.Color("#98FB98"), // Jaśniejszy zielony
			ArchiveColor:      lipgloss.Color("#FFD700"),
			ImageColor:        lipgloss.Color("#FF69B4"), // Hot Pink dla lepszego wyróżnienia
			DocumentColor:     lipgloss.Color("#98FB98"), // Jaśniejszy zielony
			DefaultFileColor:  lipgloss.Color("#E8E8E8"), // Jaśniejszy
			SelectedFileColor: lipgloss.Color("#FFA500"),

			CodeCColor:       lipgloss.Color("#FFA500"),
			CodeHColor:       lipgloss.Color("#FF8C00"),
			CodeGoColor:      lipgloss.Color("#98FB98"), // Jaśniejszy zielony
			CodePyColor:      lipgloss.Color("#FFA500"),
			CodeJsColor:      lipgloss.Color("#FFD700"), // Złoty dla lepszego kontrastu
			CodeJsonColor:    lipgloss.Color("#FF8C00"),
			CodeDefaultColor: lipgloss.Color("#E8E8E8"), // Jaśniejszy
		},
		{
			// ElectricBlue - motyw z wyrazistym elektrycznym niebieskim
			Subtle:    lipgloss.Color("#C8C8C8"), // Jaśniejszy dla lepszej widoczności
			Highlight: lipgloss.Color("#00FFFF"), // Electric Blue
			Special:   lipgloss.Color("#1E90FF"), // Dodger Blue
			Error:     lipgloss.Color("#FF6347"), // Tomato
			StatusBar: lipgloss.Color("#333333"),
			Border:    lipgloss.Color("#00FFFF"),

			ItemColor:     lipgloss.Color("#00FFFF"),
			InfotextColor: lipgloss.Color("#00FFFF"),
			HostColor:     lipgloss.Color("#1E90FF"),
			LabelColor:    lipgloss.Color("#E8E8E8"), // Jaśniejszy dla lepszej czytelności
			InputColor:    lipgloss.Color("#FFFFFF"),

			DirectoryColor:    lipgloss.Color("#1E90FF"),
			ExecutableColor:   lipgloss.Color("#90EE90"), // Jaśniejszy zielony
			ArchiveColor:      lipgloss.Color("#FFA07A"), // Light Salmon
			ImageColor:        lipgloss.Color("#FF69B4"), // Hot Pink
			DocumentColor:     lipgloss.Color("#90EE90"), // Jaśniejszy zielony
			DefaultFileColor:  lipgloss.Color("#E8E8E8"), // Jaśniejszy
			SelectedFileColor: lipgloss.Color("#00FFFF"),

			CodeCColor:       lipgloss.Color("#00FFFF"),
			CodeHColor:       lipgloss.Color("#1E90FF"),
			CodeGoColor:      lipgloss.Color("#90EE90"), // Jaśniejszy zielony
			CodePyColor:      lipgloss.Color("#00FFFF"),
			CodeJsColor:      lipgloss.Color("#FFA07A"), // Light Salmon dla kontrastu
			CodeJsonColor:    lipgloss.Color("#1E90FF"),
			CodeDefaultColor: lipgloss.Color("#E8E8E8"), // Jaśniejszy
		},
	}
)

// SwitchTheme przełącza na następny motyw i aktualizuje wszystkie style
func SwitchTheme() {
	currentThemeIndex = (currentThemeIndex + 1) % len(themes)
	currentTheme := themes[currentThemeIndex]
	updateStyles(currentTheme)
}

func updateStyles(theme Theme) {
	// Aktualizacja podstawowych kolorów
	Subtle = theme.Subtle
	Highlight = theme.Highlight
	Special = theme.Special
	Error = theme.Error
	StatusBar = theme.StatusBar
	Border = theme.Border

	// Aktualizacja wszystkich stylów
	BaseStyle = lipgloss.NewStyle().
		Foreground(Subtle).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Border)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(Highlight).
		MarginLeft(2)

	SelectedItemStyle = lipgloss.NewStyle().
		Foreground(Highlight).
		Bold(true)

	ItemStyle = lipgloss.NewStyle().
		Foreground(theme.ItemColor)

	DescriptionStyle = lipgloss.NewStyle().
		Foreground(Subtle).
		MarginLeft(2)

	Infotext = lipgloss.NewStyle().
		Foreground(theme.InfotextColor)
	InfotextStyle = Infotext

	HostStyle = lipgloss.NewStyle().
		Foreground(theme.HostColor)

	LabelStyle = lipgloss.NewStyle().
		Foreground(theme.LabelColor)

	InputStyle = lipgloss.NewStyle().
		Foreground(theme.InputColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Highlight).
		Padding(0, 1)

	StatusConnectingStyle = lipgloss.NewStyle().
		Foreground(Highlight).
		Bold(true)

	StatusConnectedStyle = lipgloss.NewStyle().
		Foreground(Special).
		Bold(true)

	StatusDefaultStyle = lipgloss.NewStyle().
		Foreground(Subtle)

	StatusStyle = lipgloss.NewStyle().
		Foreground(StatusBar)

	PanelTitleStyle = lipgloss.NewStyle().
		Foreground(Highlight).
		Bold(true).
		Padding(0, 1)

	ButtonDisabledStyle = lipgloss.NewStyle().
		Foreground(Subtle).
		Bold(true)

	DescriptionDisabledStyle = lipgloss.NewStyle().
		Foreground(Subtle).
		MarginLeft(2)

	ButtonStyle = lipgloss.NewStyle().
		Foreground(Special).
		Bold(true)

	SuccessStyle = lipgloss.NewStyle().
		Foreground(Special).
		Bold(true)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(Error).
		Bold(true)

	WindowStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(Border).
		Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
		Foreground(Highlight).
		Bold(true).
		Underline(true).
		Padding(0, 1)

	CellStyle = lipgloss.NewStyle().
		Foreground(theme.InputColor).
		Padding(0, 1)

	DialogStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Padding(1, 2)

	DialogTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(Highlight).
		Padding(0, 1)

	DialogButtonStyle = lipgloss.NewStyle().
		Foreground(Special).
		Bold(true).
		Padding(0, 2)

	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(Border).
		Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
		Foreground(theme.InputColor).
		Background(StatusBar).
		Bold(true).
		Padding(0, 1).
		Width(103)

	CommandBarStyle = lipgloss.NewStyle().
		Foreground(theme.InputColor).
		Padding(0, 0).
		Width(103).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(Border)

	// Style dla plików
	DirectoryStyle = lipgloss.NewStyle().
		Foreground(theme.DirectoryColor).
		Bold(true)

	ExecutableStyle = lipgloss.NewStyle().
		Foreground(theme.ExecutableColor)

	ArchiveStyle = lipgloss.NewStyle().
		Foreground(theme.ArchiveColor)

	ImageStyle = lipgloss.NewStyle().
		Foreground(theme.ImageColor)

	DocumentStyle = lipgloss.NewStyle().
		Foreground(theme.DocumentColor)

	// Style dla kodu
	CodeCStyle = lipgloss.NewStyle().
		Foreground(theme.CodeCColor)

	CodeHStyle = lipgloss.NewStyle().
		Foreground(theme.CodeHColor)

	CodeGoStyle = lipgloss.NewStyle().
		Foreground(theme.CodeGoColor)

	CodePyStyle = lipgloss.NewStyle().
		Foreground(theme.CodePyColor)

	CodeJsStyle = lipgloss.NewStyle().
		Foreground(theme.CodeJsColor)

	CodeJsonStyle = lipgloss.NewStyle().
		Foreground(theme.CodeJsonColor)

	CodeDefaultStyle = lipgloss.NewStyle().
		Foreground(theme.CodeDefaultColor)

	DefaultFileStyle = lipgloss.NewStyle().
		Foreground(theme.DefaultFileColor)

	SelectedFileStyle = lipgloss.NewStyle().
		Foreground(theme.SelectedFileColor)
}
