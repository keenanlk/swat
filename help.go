package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/term"
)

// The guides in docs/ are what GitHub shows and what `swat help` prints.
//
//go:embed docs/*.md
var docsFS embed.FS

type topic struct {
	name, file, summary string
	aliases             []string
}

var topics = []topic{
	{"getting-started", "getting-started.md", "install, first run, your first agent", []string{"start", "install", "setup"}},
	{"usage", "usage.md", "the dashboard, keys, statuses, and the agent workflow", []string{"keys", "dashboard"}},
	{"config", "configuration.md", "every setting in ~/.config/swat/config.json, with recipes", []string{"configuration", "settings", "repos"}},
	{"project", "project-config.md", ".swat.json: per-repo settings you commit for your team", []string{".swat.json", "swat.json", "init"}},
	{"troubleshooting", "troubleshooting.md", "fixes for common problems", []string{"doctor", "problems", "faq"}},
	{"how-it-works", "how-it-works.md", "what swat runs, and the files it reads and writes", []string{"internals", "files"}},
}

func findTopic(name string) (topic, bool) {
	name = strings.ToLower(name)
	for _, t := range topics {
		if t.name == name {
			return t, true
		}
		for _, a := range t.aliases {
			if a == name {
				return t, true
			}
		}
	}
	return topic{}, false
}

// color wraps s in an ANSI SGR code when stdout is a terminal.
func color(code, s string) string {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func docText(file string) string {
	data, _ := docsFS.ReadFile("docs/" + file)
	return string(data)
}

func helpCmd(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		fmt.Println("\nHelp topics (swat help <topic>):")
		for _, t := range topics {
			fmt.Printf("  %-16s %s\n", t.name, t.summary)
		}
		return nil
	}
	t, ok := findTopic(args[0])
	if !ok {
		return fmt.Errorf("no help topic %q (run `swat help` for the list)", args[0])
	}
	fmt.Print(renderMarkdown(docText(t.file), 0))
	return nil
}

// renderMarkdown styles markdown for a terminal, or returns it as-is when
// output isn't a terminal (so it pipes cleanly). width 0 = terminal width.
func renderMarkdown(md string, width int) string {
	if width == 0 {
		if !term.IsTerminal(os.Stdout.Fd()) {
			return md
		}
		width = 100
		if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
			width = min(w, 110)
		}
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(width-2))
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return out
}
