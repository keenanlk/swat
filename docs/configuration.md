# Configuration

swat works with **no configuration at all**. Everything below is optional.

There are two places for settings:

| File | Whose settings | Committed? |
|---|---|---|
| `~/.config/swat/config.json` | Yours: your search filter, model, editor, Claude account | No, it's personal |
| `.swat.json` at a repo's root | The repo's: where its issues come from, extra agent instructions | Yes, see [Project settings](project-config.md) |

If you set `$XDG_CONFIG_HOME`, swat uses `$XDG_CONFIG_HOME/swat/` instead of
`~/.config/swat/`.

## Seeing and changing settings

```sh
swat config                         # every setting, its value, and where it came from
swat config init                    # create a starter file with every setting documented
swat config edit                    # open the file ($VISUAL, $EDITOR, or your editor)
swat config set permissionMode auto # change one setting
swat config unset permissionMode    # back to the default
swat config path                    # where the file is
```

`swat config` labels each value as **[config]** (from your file),
**[default]** or **[detected]** (worked out from your machine).

The config file is JSON, and it may contain `//` and `/* */` comments.
`swat config init` writes every setting commented out with its explanation, so
you can uncomment what you need. `swat config set` won't modify a file that
contains your own comments, because it would lose them. Use
`swat config edit` for that file instead.

## Settings

### `query`

Extra [GitHub search qualifiers](https://docs.github.com/en/search-github/searching-on-github/searching-issues-and-pull-requests)
added to the issue search. swat always searches for:

```
is:issue is:open archived:false assignee:<you>
```

and appends your `query`. Examples:

| `query` | Shows |
|---|---|
| `"org:acme"` | only issues in the acme organization |
| `"repo:acme/issues"` | only issues in one repo |
| `"org:acme label:bug"` | only bugs in acme |
| `"-label:wontfix"` | everything except issues labelled wontfix |
| `"repo:acme/api repo:acme/web"` | issues in either repo |

Default: none, so all of your assigned issues in every repo you can see.
swat shows up to 100 issues, the most recently updated first.

### `permissionMode`

Passed to Claude Code as `--permission-mode`. Possible values:
`acceptEdits`, `auto`, `bypassPermissions`, `dontAsk`, `manual` and `plan`.
See the Claude Code docs for exactly what each one allows.

This matters more for background agents than for normal sessions. When an
agent needs permission (to run a command, say), it **stops and waits** until
you open it (`a`) and answer. If you'd rather agents keep going:

- `auto` lets Claude Code approve safe actions itself. It's the best default
  for swat if your account has it.
- `acceptEdits` approves file edits automatically, but still asks before
  running commands.

Default: whatever your Claude Code settings use.

### `model`

Passed to Claude Code as `--model`, for example `"opus"` or `"sonnet"`.
Default: your Claude Code default.

### `pauseAfterPlan`

Whether the **pause for my approval after the plan** box starts ticked. You
can always toggle it per agent with `space` in the start form.
Default: `true`.

### `editor`

The command that `o` uses to open a folder; swat adds the folder's path to the
end. Examples: `"code"`, `"cursor"`, `"zed"`, `"phpstorm"`, `"idea"`,
`"open -a Fork"`.

Default: the first of `cursor`, `code`, `zed`, `phpstorm`, `idea`,
`webstorm`, `goland`, `pycharm` and `subl` that's on your PATH. If none are,
swat uses `open` on macOS or `xdg-open` on Linux.

### `searchPaths`

The folders swat scans for local clones. It looks two levels deep, so both
`~/code/api` and `~/code/acme/api` are found. Only main clones count (not
worktrees), and only ones whose `origin` remote is on github.com.

Setting this **replaces** the built-in list, which is:

```
~/code ~/src ~/dev ~/projects ~/Projects ~/repos ~/git ~/github ~/Developer
~/workspace ~/work ~/go/src/github.com ~/PhpstormProjects ~/WebstormProjects
~/IdeaProjects ~/GolandProjects ~/PycharmProjects
```

From the command line, separate paths with commas:
`swat config set searchPaths "~/code, ~/clients"`.

### `repos`

Clones to always offer, and to rank first, in the start form. Map each GitHub
`owner/name` to its local path:

```json
"repos": {
  "acme/api": "~/work/api",
  "acme/web": "/Volumes/dev/acme-web"
}
```

Use this for clones outside your search paths, or to make sure a repo always
comes first. You usually don't need it, because swat also **remembers** every
clone you start an agent in. Those are kept in `~/.config/swat/repos.json`,
and you can manage them from the command line:

```sh
swat repos                  # every clone swat knows about
swat repos add ~/work/api   # pin a clone
swat repos forget acme/api  # unpin it
```

### `claudeBin`

The `claude` executable to run. Default: `claude` from your PATH, or
`~/.local/bin/claude` if it isn't on your PATH.

### `claudeConfigDir`

Runs agents with this `CLAUDE_CONFIG_DIR`. Use it when you keep a second
Claude Code login in its own config directory (see the recipe below).
Default: unset, so agents use your normal Claude Code login and settings.

## Recipes

### Use a second Claude account for agents

If you run a separate Claude Code login with something like
`CLAUDE_CONFIG_DIR=~/.claude-work claude`:

```json
{
  "claudeConfigDir": "~/.claude-work"
}
```

swat then runs, lists, opens and resumes every agent under that login.
Repos have to be trusted under that login too, so run
`CLAUDE_CONFIG_DIR=~/.claude-work claude` in them once.

### Your team tracks issues in a separate repo

For example, issues in `acme/issues` and code in `acme/api` and `acme/web`:

1. Filter to the tracker, if you only want to see those issues:

   ```json
   { "query": "repo:acme/issues" }
   ```

2. In each code repo, commit a `.swat.json` naming the tracker, so swat
   suggests the code repos when you start an agent:

   ```json
   { "issueRepos": ["acme/issues"] }
   ```

   See [Project settings](project-config.md).

### Only bugs, only at work

```json
{ "query": "org:acme label:bug" }
```

### Let agents run without stopping for permissions

```json
{ "permissionMode": "auto" }
```

### A full example

```jsonc
{
  // Work issues only
  "query": "org:acme",
  "permissionMode": "auto",
  "model": "opus",
  "editor": "phpstorm",
  "claudeConfigDir": "~/.claude-work",
  "searchPaths": ["~/PhpstormProjects"],
  "repos": { "acme/legacy-api": "/Volumes/dev/legacy-api" }
}
```

## GitHub credentials

swat uses `GITHUB_TOKEN` if it's set, and otherwise `gh auth token` (the
GitHub CLI's login). Pushing with `p` uses your normal git credentials, not
the token.
