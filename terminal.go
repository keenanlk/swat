package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var errUnknownTerminal = errors.New("don't know how to open a window in this terminal")

// terminalName describes where openTerminal will open windows, or "" when
// it can't and swat attaches in place instead.
func terminalName() string {
	switch {
	case os.Getenv("TMUX") != "":
		return "tmux"
	case os.Getenv("TERM_PROGRAM") == "Apple_Terminal":
		return "Terminal"
	case os.Getenv("TERM_PROGRAM") == "iTerm.app":
		return "iTerm2"
	case os.Getenv("TERM_PROGRAM") == "ghostty":
		return "Ghostty"
	case os.Getenv("TERM_PROGRAM") == "WezTerm":
		return "WezTerm"
	}
	return ""
}

// openTerminal runs cmdline in a new window of the terminal we're running in.
func openTerminal(cmdline string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	var cmd *exec.Cmd
	switch terminalName() {
	case "tmux":
		cmd = exec.Command("tmux", "new-window", "-n", "swat", shell, "-lc", cmdline)
	case "Terminal":
		cmd = exec.Command("osascript",
			"-e", fmt.Sprintf(`tell application "Terminal" to do script %s`, appleString(cmdline)),
			"-e", `tell application "Terminal" to activate`)
	case "iTerm2":
		cmd = exec.Command("osascript",
			"-e", `tell application "iTerm2" to create window with default profile`,
			"-e", fmt.Sprintf(`tell application "iTerm2" to tell current session of current window to write text %s`, appleString(cmdline)))
	case "Ghostty":
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", "-na", "Ghostty.app", "--args", "-e", shell, "-lc", cmdline)
		} else {
			cmd = exec.Command("ghostty", "-e", shell, "-lc", cmdline)
		}
	case "WezTerm":
		cmd = exec.Command("wezterm", "cli", "spawn", "--new-window", "--", shell, "-lc", cmdline)
	default:
		return errUnknownTerminal
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("opening a terminal window: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func openURL(u string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", u).Run()
	}
	return exec.Command("xdg-open", u).Run()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func appleString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
