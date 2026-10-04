// swat lists the GitHub issues assigned to you and runs a background
// Claude Code agent per issue, each in its own git worktree.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// version is set at release time.
var version = "dev"

const usage = `swat: send Claude Code agents after the GitHub issues assigned to you.

Usage:
  swat                         open the dashboard (press ? inside for help)
  swat doctor                  check your setup and say how to fix problems

  swat config                  show every setting and where its value comes from
  swat config init             create a starter config file with every option documented
  swat config edit             open the config file in your editor
  swat config set <key> <val>  change one setting
  swat config unset <key>      reset one setting to its default
  swat config path             print the config file's location

  swat repos                   list the local clones swat can work in
  swat repos add <path>        pin a clone so it's always offered first
  swat repos forget <name>     unpin a clone

  swat init                    create a .swat.json in the current repo
  swat help [topic]            guides: getting-started, usage, config, project,
                               troubleshooting, how-it-works
  swat version                 print the version
`

func main() {
	cmd, args := "", os.Args[1:]
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	// help and version work even when the config file is broken.
	switch cmd {
	case "help", "--help", "-h":
		check(helpCmd(args))
		return
	case "version", "--version", "-v":
		fmt.Println(version)
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		// Let people fix or inspect a broken config file.
		if cmd == "config" && len(args) > 0 && (args[0] == "edit" || args[0] == "path" || args[0] == "init") {
			cfg = Config{Repos: map[string]string{}, Remembered: map[string]string{}}
		} else {
			fail(fmt.Errorf("reading %s: %w", configPath(), err))
		}
	}
	switch cmd {
	case "":
		check(run(cfg))
	case "doctor":
		os.Exit(doctor(cfg))
	case "config":
		check(configCmd(cfg, args))
	case "repos":
		check(reposCmd(cfg, args))
	case "init":
		check(initProject())
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

func check(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "swat:", err)
	os.Exit(1)
}

func run(cfg Config) error {
	st, err := loadState()
	if err != nil {
		return fmt.Errorf("reading %s: %w", statePath(), err)
	}
	gh, err := newGitHub()
	if err != nil {
		return err
	}
	who, err := gh.login()
	if err != nil {
		return fmt.Errorf("checking your GitHub login: %w (try `swat doctor`)", err)
	}
	_, err = tea.NewProgram(newModel(cfg, gh, st, who), tea.WithAltScreen()).Run()
	return err
}
