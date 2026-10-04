# How it works

swat doesn't run agents itself. It's a dashboard over Claude Code's
background sessions and git worktrees. Here's exactly what it runs and which
files it touches.

## Finding issues

swat calls GitHub's issue search API:

```
is:issue is:open archived:false assignee:<you> <your query>
```

`<you>` is the login that your token belongs to. swat fetches the issue's
comments only when you start an agent for it.

## Starting an agent

When you press `enter` in the start form, swat:

1. Adds `.swat/` to the clone's `.git/info/exclude`. That file is local to
   your machine and isn't committed. It keeps the agent's notes out of
   `git status` and out of commits, in every worktree of the clone.
2. Runs, from your clone:

   ```sh
   claude --bg -w swat-<number> -n <owner/repo#number> \
     [--permission-mode <permissionMode>] [--model <model>] "<prompt>"
   ```

   - `--bg` starts a background session.
   - `-w` makes Claude Code create a worktree at
     `<clone>/.claude/worktrees/swat-<number>`, on a new branch named
     `worktree-swat-<number>`. If that worktree already exists, swat adds
     `-2`, `-3` and so on to the name.

3. Records the agent in `~/.config/swat/state.json`.

The prompt contains the issue (title, labels, URL, body and comments), the
step-by-step workflow, the rules (no pushing, stay in the worktree), and the
repo's `.swat.json` instructions. The prompt tells the agent to treat the
issue text as a description of the problem, not as instructions. That makes
it harder for text in an issue to steer the agent, but it isn't a guarantee,
so review plans and diffs from issues you don't trust.

## Following an agent

Every 2 seconds, swat:

- runs `claude agents --json --all` to get each agent's status (busy, idle or
  stopped)
- reads `.swat/status.json` from each worktree to get the agent's stage and note
- for the selected issue only:
  - reads `.swat/PLAN.md`
  - reads the agent's session transcript (`<claude config dir>/projects/*/<session-id>.jsonl`) for the log pane
  - runs `git rev-list` and `git diff --shortstat` against `origin/HEAD` for
    the commit count and diff

The agent writes the `.swat/` files itself, because its prompt tells it to.

## Messaging and approving

Resuming an idle background session normally starts a *copy* of it. So for
`y` and `m`, swat runs `claude stop <id>` and then
`claude --bg --resume <session-id> "<message>"`. That wakes the original
session, with its saved options, under the same ID.

## Other actions

| Key | Runs |
|---|---|
| `a` | `claude attach <id>` in a new terminal window, using AppleScript (Terminal, iTerm2), `open -na Ghostty`, `wezterm cli spawn`, or `tmux new-window` |
| `x` | `claude stop <id>` |
| `D` | `claude rm <id>`, which deletes the session and its worktree when that's safe |
| `t` | your `$SHELL` in the worktree |
| `o` | your editor with the worktree's path |
| `c` | `claude stop`; copy `.swat/` to `~/.config/swat/artifacts/`; `git worktree unlock` and `git worktree remove`; `git checkout <branch>` in the main clone |
| `p` | `git push -u origin <branch>`, then open `https://github.com/<repo>/compare/<branch>?expand=1` |

## Files

| Path | Written by | Contents |
|---|---|---|
| `~/.config/swat/config.json` | you (or `swat config set`) | your settings |
| `~/.config/swat/repos.json` | swat | clones you've used or pinned |
| `~/.config/swat/state.json` | swat | your agents: issue, repo, worktree, branch, session ID |
| `~/.config/swat/artifacts/` | swat | saved plans and summaries for branches checked out with `c` |
| `<repo>/.swat.json` | your team | project settings |
| `<worktree>/.swat/` | the agent | `status.json`, `PLAN.md`, `SUMMARY.md` |
| `<clone>/.git/info/exclude` | swat (one line) | `.swat/` |

swat never writes outside these files, except through the git and claude
commands listed above.

## Clone discovery

swat looks through every folder in `searchPaths` (or the built-in list), two
levels deep, for a `.git` *directory*. That skips worktrees, which have a
`.git` file instead. For each one, it reads `remote "origin"` from
`.git/config` and keeps clones whose remote is on github.com. Configured and
remembered repos are added on top and marked as pinned.

## Trust check

`swat doctor` reads Claude Code's `~/.claude.json` (or
`$CLAUDE_CONFIG_DIR/.claude.json`) to see which folders you've accepted the
trust prompt for. This is Claude Code's internal file, so when swat can't
read it, doctor simply skips the check.
