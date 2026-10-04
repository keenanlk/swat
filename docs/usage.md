# Using the dashboard

Run `swat`. The screen has three parts:

- **Header:** your GitHub login, any search filter, and counts of running
  agents, agents that need you, and issues with no agent.
- **Issues (left):** your open, assigned issues, most recently updated first.
  Under each title is the issue's repo, then `→ repo` if the fix is being made
  in a different repo, then the issue's labels.
- **Detail (right):** for an issue with no agent, its description. For an
  issue with an agent: its stages, branch, worktree, diff, plan checklist and
  a live log of what it's doing.

Press **`?`** at any time to read this guide inside swat.

## Keys

### Moving around

| Key | Action |
|---|---|
| `↑` `↓` (or `k` `j`) | Select an issue |
| `r` | Refresh the issue list from GitHub |
| `?` | Help |
| `q` | Quit (agents keep running) |

### Agents

| Key | Action |
|---|---|
| `s` | Start an agent for the selected issue |
| `enter` / `a` | Open the agent's Claude Code session in a new terminal window |
| `A` | Open the agent's session in this window (swat comes back when it exits) |
| `v` | Read the full plan |
| `y` | Approve the plan (when the status is `◆ plan ready`) |
| `m` | Send the agent a message |
| `x` | Stop the agent. Its worktree and branch stay; `m` wakes it again |
| `D` | Delete the agent and its worktree. The branch and its commits stay |

### Your code

| Key | Action |
|---|---|
| `t` | Open a shell in the agent's worktree. Type `exit` to come back |
| `o` | Open the worktree in your editor |
| `c` | Check the agent's branch out in your main clone (see below) |
| `p` | Push the branch to `origin` and open GitHub's pull request page |

`x`, `D`, `c` and `p` ask you to confirm first.

### In the start form

| Key | Action |
|---|---|
| `↑` `↓` | Choose the clone to fix the issue in |
| `space` | Toggle **pause for my approval after the plan** |
| `enter` | Start the agent. On **another folder…**, type a path to any clone |
| `esc` | Cancel |

### In the plan viewer

| Key | Action |
|---|---|
| `↑` `↓` `pgup` `pgdn` | Scroll |
| `y` | Approve the plan |
| `m` | Ask for changes |
| `esc` / `v` / `q` | Back to the list |

## Statuses

| Status | Meaning | What to do |
|---|---|---|
| `○ idle` | No agent for this issue | `s` to start one |
| `● starting` | The agent has launched but hasn't reported a stage yet | Wait |
| `● research 4m` | Working. Shows the stage and time since it started | Wait, or `a` to watch |
| `◆ plan ready` | The plan is written and the agent is waiting for you | `v` to read, `y` to approve, `m` to ask for changes |
| `◆ blocked` | The agent needs a decision. Its question is in the **note** line | `m` to answer |
| `◆ idle · <stage>` | The agent stopped mid-way. Usually it's waiting on a permission prompt | `a` to open it and answer |
| `✓ ready` | Fix committed, summary written | Test it with `t`, `o` or `c`, then `p` |
| `✓ checked out` | You checked the branch out in your main clone | Test it, then `p` |
| `✕ stopped` | The agent isn't running (you stopped it, or it was removed) | `m` to wake it, or `D` to clean up |

## The agent's workflow

Every agent gets the issue's title, body, labels and comments, plus these
instructions:

1. **research:** read the code, find the root cause, and reproduce the bug
   with a test if it reasonably can.
2. **plan:** write a plan (root cause plus a checklist) to `.swat/PLAN.md`.
3. If you left **pause after plan** on, set the status to `plan ready` and wait for you.
4. **implement:** make the fix, add or update tests, run the relevant tests
   and linters, and commit on its branch.
5. **ready:** write `.swat/SUMMARY.md` (what changed, why, how to test it
   by hand) and finish.

Agents never push, open pull requests, or touch anything outside their
worktree. If one gets stuck, it marks itself **blocked** with a question.

Repos can add their own instructions, such as which test command to run. See
[Project settings](project-config.md).

## Talking to an agent

**`m`** sends a message. If the agent is idle or stopped, swat stops its
background session and resumes it with your message, so the agent keeps its
full context. If the agent is busy, swat won't interrupt it. Press **`a`** to
open the session and type to it directly.

**`a`** opens the agent's live Claude Code session in a new terminal window.
That's where you answer permission prompts and watch it work. Closing that
window doesn't stop the agent. New windows work in Terminal, iTerm2, Ghostty,
WezTerm and tmux. In other terminals, swat opens the session in its own
window and comes back when the session exits.

## Testing a fix

You have three options, from lightest to heaviest:

1. **`t`**: a shell in the worktree. Run the app or the tests right there.
   The worktree is a full checkout on the agent's branch.
2. **`o`**: open the worktree in your editor.
3. **`c`**: check the branch out in your **main clone**. Use this when your
   dev setup only works from the main folder (Docker volumes, IDE run
   configurations, and so on). git doesn't allow one branch to be checked out
   in two places, so swat:
   1. stops the agent
   2. saves a copy of its plan and summary
   3. removes the worktree, which fails if the worktree has uncommitted changes
   4. runs `git checkout <branch>` in your main clone, which fails if your main clone has uncommitted changes

   After that, the issue shows `✓ checked out`, and `t`, `o` and `p` work on
   your main clone.

## Cleaning up

- **`x`** stops an agent but keeps its worktree, so you can wake it later with `m`.
- **`D`** deletes the agent's session and its worktree. The branch, and any
  commits on it, stay in your clone. Delete the branch with
  `git branch -D worktree-swat-<number>` once it's merged or no longer needed.
