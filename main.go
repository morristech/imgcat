package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
	"github.com/nfnt/resize"
)

const usage = `imgcat [pattern|url]

Examples:
    imgcat path/to/image.jpg
    imgcat *.jpg
    imgcat https://example.com/image.jpg`

func main() {
	if len(os.Args) == 1 {
		fmt.Println(usage)
		os.Exit(1)
	}

	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Println(usage)
		os.Exit(0)
	}

	p := tea.NewProgram(model{urls: os.Args[1:len(os.Args)]})
	p.EnterAltScreen()
	defer p.ExitAltScreen()
	if err := p.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

const sparkles = "✨"

type model struct {
	selected int
	urls     []string
	image    string
	height   uint
	err      error
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = uint(msg.Height)
		return m, load(m.urls[m.selected])
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "j", "down":
			if m.selected+1 != len(m.urls) {
				m.selected++
			} else {
				m.selected = 0
			}
			return m, load(m.urls[m.selected])
		case "k", "up":
			if m.selected-1 != -1 {
				m.selected--
			} else {
				m.selected = len(m.urls) - 1
			}
			return m, load(m.urls[m.selected])
		}
	case errMsg:
		m.err = msg
		return m, nil
	case loadMsg:
		url := m.urls[m.selected]
		if msg.resp != nil {
			defer msg.resp.Body.Close()
			img, err := readerToImage(m.height, url, msg.resp.Body)
			if err != nil {
				return m, func() tea.Msg { return errMsg{err} }
			}
			m.image = img
			return m, nil
		}
		defer msg.file.Close()
		img, err := readerToImage(m.height, url, msg.file)
		if err != nil {
			return m, func() tea.Msg { return errMsg{err} }
		}
		m.image = img
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("couldn't load image(s): %v\n\npress any key to exit", m.err)
	}
	if m.image == "" {
		return fmt.Sprintf("loading %s %s", m.urls[m.selected], sparkles)
	}
	return m.image
}

type loadMsg struct {
	resp *http.Response
	file *os.File
}

type errMsg struct{ error }

func load(url string) tea.Cmd {
	if strings.HasPrefix(url, "http") {
		return func() tea.Msg {
			resp, err := http.Get(url)
			if err != nil {
				return errMsg{err}
			}
			return loadMsg{resp: resp}
		}
	}
	return func() tea.Msg {
		file, err := os.Open(url)
		if err != nil {
			return errMsg{err}
		}
		return loadMsg{file: file}
	}
}

func readerToImage(height uint, url string, r io.Reader) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", err
	}

	img = resize.Resize(0, height*2, img, resize.Lanczos3)
	b := img.Bounds()
	w := b.Max.X
	h := b.Max.Y
	p := termenv.ColorProfile()
	str := strings.Builder{}
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			c1, _ := colorful.MakeColor(img.At(x, y))
			color1 := p.Color(c1.Hex())
			c2, _ := colorful.MakeColor(img.At(x, y+1))
			color2 := p.Color(c2.Hex())
			str.WriteString(termenv.String("▀").
				Foreground(color1).
				Background(color2).
				String())
		}
		str.WriteString("\n")
	}
	str.WriteString(fmt.Sprintf("q to quit | %s\n", url))
	return str.String(), nil
}
