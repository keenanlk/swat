# Troubleshooting

Start with:

```sh
swat doctor
```

It checks Claude Code, your GitHub login, git, your editor and terminal, and
your clones, and says how to fix whatever is wrong.

## Starting agents

### "Workspace not trusted"

Claude Code only runs agents in folders you've approved. Run Claude Code in
the repo once and accept the prompt:

```sh
cd ~/code/my-repo && claude
```

If you use `claudeConfigDir`, trust the repo under that login:
`CLAUDE_CONFIG_DIR=~/.claude-work claude`. `swat doctor` and `swat repos`
mark untrusted clones.

### "this claude has no --bg/--worktree"

Your Claude Code is too old for background agents. Run `claude update`.

### The repo I want isn't in the start form

swat only finds clones under its search paths, two levels deep, whose
`origin` remote is on github.com. Any of these will fix it:

- choose **another folder…** in the start form and type the path
- `swat repos add ~/path/to/clone`
- add the folder that contains it to `searchPaths` (see [Configuration](configuration.md))

Clones of GitHub Enterprise or non-GitHub remotes aren't supported.

### The wrong repo is suggested first

Add `.swat.json` with `issueRepos` to the right repo (see
[Project settings](project-config.md)), or pin it with `swat repos add`.

## Agents that stop

### An agent shows `◆ idle · research` (or another stage) and isn't moving

It's most likely waiting on a permission prompt. Press `a` to open it and
answer. To avoid this, set `permissionMode` to `auto` or `acceptEdits`:

```sh
swat config set permissionMode auto
```

### `m` says the agent is busy

swat won't interrupt a working agent. Press `a` to open its session and type
to it there, or wait until it's idle.

### Opening an agent (`a`) doesn't open a new window

New windows work in Terminal, iTerm2, Ghostty, WezTerm and tmux. In other
terminals, swat opens the agent in its own window instead, and comes back
when the session exits. On macOS, the first time you press `a`, allow your
terminal to control Terminal or iTerm2 when macOS asks.
`swat doctor` shows which method it will use.

## Issues

### No issues show up

- Check the search: `swat config` prints the exact query. Make sure your
  `query` isn't too narrow.
- The issues must be **open** and **assigned to you**.
- For private repos, your token needs access (see [Getting started](getting-started.md)).
- Press `r` to refresh; swat doesn't re-fetch on its own.

### "GitHub: 401" or "403"

Your token is missing, expired or lacks access. Run `gh auth login`, or check
`GITHUB_TOKEN`. A 403 that mentions a rate limit clears within a minute.

## Testing and shipping

### `c` (check out) fails with "contains modified or untracked files"

The agent's worktree has uncommitted changes. Open it with `t`, then commit
or discard the changes, and try again.

### `c` fails with "Your local changes to the following files would be overwritten by checkout"

Your **main clone** has uncommitted changes. Commit or stash them first, or
test in the worktree with `t` instead.

### `p` (push) fails

swat runs `git push -u origin <branch>` with your normal git credentials.
Run the same command in a shell (`t`) to see the full error.

## Starting over

- Delete one agent and its worktree: select it and press `D`.
- Forget everything swat knows (agents keep running in Claude Code):
  `rm ~/.config/swat/state.json`
- See every background agent, including ones swat no longer tracks:
  `claude agents`

## Reporting a bug

Please include the output of `swat doctor` and `swat version`, at
https://github.com/keenanlk/swat/issues.
