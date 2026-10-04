# Getting started

swat lists the GitHub issues assigned to you. For any of them, it can start a
Claude Code agent that researches the problem, writes a plan, and makes the
fix on its own branch in its own git worktree. You review the plan, check the
branch out when you want to test it, and push it when you're happy.

## 1. Install

```sh
brew install --cask keenanlk/tap/swat
```

or, with Go 1.24+:

```sh
go install github.com/keenanlk/swat@latest
```

swat runs on macOS and Linux.

## 2. Make sure you have the requirements

| You need | Why | How |
|---|---|---|
| **Claude Code**, logged in | swat runs your agents as Claude Code background sessions | [Install Claude Code](https://docs.claude.com/en/docs/claude-code), run `claude` once to log in, and `claude update` if it's old |
| **GitHub access** | to find your issues and read their comments | `gh auth login` ([GitHub CLI](https://cli.github.com)), or set `GITHUB_TOKEN` |
| **git** | worktrees, branches, pushing | already installed on most machines |
| **Local clones** of the repos you fix bugs in | agents work in a worktree of your clone | `git clone` as usual, ideally under `~/code`, `~/src`, `~/dev` or `~/projects` |

If you use `GITHUB_TOKEN` and your repos are private, the token needs read
access to their issues. With a classic token, that's the `repo` scope. With a
fine-grained token, it's **Issues: read** and **Metadata: read**.

## 3. Check your setup

```sh
swat doctor
```

doctor checks Claude Code, your GitHub login, git, your editor and terminal,
and the clones swat found. If anything is wrong, it tells you how to fix it.

The most common issue on first run is a **repo Claude Code doesn't trust
yet**. Claude Code only runs agents in folders you've approved. Open the repo
once with `claude`, accept the trust prompt, and quit:

```sh
cd ~/code/my-repo && claude
```

## 4. Run it

```sh
swat
```

You'll see your open, assigned issues on the left. Use `↑`/`↓` to pick one;
the right side shows its description.

## 5. Send an agent after a bug

1. Select an issue and press **`s`**.
2. Pick the clone to fix it in. swat lists your clone of the issue's repo
   first. If issues live in a separate tracker repo, see
   [Project settings](project-config.md).
3. Leave **pause for my approval after the plan** ticked (`space` toggles it)
   and press **`enter`**.

The agent starts in the background. Its status in the list goes from
`● starting` to `● research` to `◆ plan ready`.

## 6. Review the plan

When the status is `◆ plan ready`, press **`v`** to read the plan. Then:

- **`y`** approves it, and the agent implements the fix.
- **`m`** sends a message instead ("also handle the mobile view").

## 7. Test the fix

When the status is `✓ ready`, the agent has committed its fix and written a
summary. You can:

- press **`t`** for a shell in the agent's worktree, to run the app or tests there
- press **`o`** to open the worktree in your editor
- press **`c`** to check the branch out in your main clone instead

## 8. Ship it

Press **`p`** to push the branch and open GitHub's "create pull request" page.

## Next

- [Using the dashboard](usage.md): every key, and what each status means
- [Configuration](configuration.md): settings and recipes, such as using a
  second Claude account or filtering to one org
- [Project settings](project-config.md): `.swat.json` for your team's repos
- [Troubleshooting](troubleshooting.md)

All of these guides are also available offline, with `swat help <topic>`.
