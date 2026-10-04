# swat

**Send Claude Code agents after the GitHub issues assigned to you.**

swat is a terminal dashboard. It lists your open, assigned issues and lets
you start a [Claude Code](https://docs.claude.com/en/docs/claude-code) agent
for any of them. The agent researches the bug, writes a plan, waits for you
to approve it if you want, then fixes the bug and commits on its own branch in
its own git worktree. Several agents can run at once without touching your
checkout. You review, test and push when you're ready.

```
 swat  @you                                          ● 2 running  ◆ 1 needs you  ○ 3 idle
╭─ Issues (6) ──────────────────────────╮ ╭─ acme/api#212 · Dates show raw ISO strings ──────╮
│ ▸ #212   ◆ plan ready   Dates show r… │ │ ✓ research ── ◆ plan (needs you) ── ○ implement  │
│          acme/api · bug               │ │ branch   worktree-swat-212   0 commits ahead     │
│   #198   ● impl 14m     Export times… │ ╰──────────────────────────────────────────────────╯
│          acme/issues → acme/web       │ ╭─ Plan ready for review · v to read · y to approve╮
╰───────────────────────────────────────╯ ╰──────────────────────────────────────────────────╯
```

## Quick start

```sh
brew install --cask keenanlk/tap/swat   # or: go install github.com/keenanlk/swat@latest
swat doctor                             # check your setup
swat                                    # open the dashboard; press ? for help
```

You need Claude Code (logged in), GitHub access (`gh auth login` or
`GITHUB_TOKEN`), git, and local clones of the repos you work on. swat runs on
macOS and Linux.

In the dashboard: select an issue, press **`s`** to start an agent, **`v`**
to read its plan, **`y`** to approve it, **`t`** to test the fix in a shell,
and **`p`** to push and open a pull request.

## Configuration

None is needed. When you want to change something:

```sh
swat config                          # every setting, its value, and where it comes from
swat config set permissionMode auto  # let agents work without stopping for permissions
swat config set query "org:acme"     # only show issues from one org
swat config edit                     # edit the (commentable) config file
swat repos add ~/work/api            # offer a clone outside the usual folders
```

For a team, commit a `.swat.json` to a repo (`swat init` creates one). Use it
to point a separate issue-tracker repo at the code repo, and to give agents
project-specific instructions:

```json
{
  "issueRepos": ["acme/issues"],
  "instructions": "Run `make test` before committing."
}
```

## Documentation

| Guide | |
|---|---|
| [Getting started](docs/getting-started.md) | install, requirements, your first agent |
| [Using the dashboard](docs/usage.md) | every key, what each status means, testing and shipping fixes |
| [Configuration](docs/configuration.md) | every setting, with recipes (second Claude account, org filters, …) |
| [Project settings](docs/project-config.md) | `.swat.json` for your team's repos |
| [Troubleshooting](docs/troubleshooting.md) | fixes for common problems |
| [How it works](docs/how-it-works.md) | the exact commands swat runs and the files it touches |

The same guides are built into swat. Run `swat help <topic>`, or press `?`
in the dashboard.

## License

[MIT](LICENSE)
