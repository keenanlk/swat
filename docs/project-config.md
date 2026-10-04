# Project settings: `.swat.json`

A repo can include a `.swat.json` file at its root. It tells swat, and the
agents it starts, about that project. Commit it, and everyone who uses swat
on the repo gets the same setup.

Create a starter file in the repo you're in with:

```sh
swat init
```

## Example

```json
{
  "issueRepos": ["acme/issues"],
  "instructions": "Run `make test` and `make lint` before committing. Follow CONTRIBUTING.md. Never edit files under generated/."
}
```

## Fields

### `issueRepos`

A list of other repos (`owner/name`) whose issues are usually fixed in this
repo.

When you start an agent, swat ranks the clones it could work in like this:

1. your clone of the issue's own repo
2. clones whose `.swat.json` lists the issue's repo in `issueRepos`
3. other clones from the same owner
4. everything else

Within each group, clones you've configured or used before come first.

So if your team tracks bugs in `acme/issues` but fixes them in `acme/api`,
add `"issueRepos": ["acme/issues"]` to `acme/api`'s `.swat.json`. Then
`acme/api` is suggested for every `acme/issues` bug.

You don't need this field when a repo's issues are fixed in the repo itself.

### `instructions`

Text added to the end of every agent prompt for this repo. Good things to
put here:

- how to run the tests and linters (`"Run npm test -- --related"`)
- conventions agents should follow (`"Use the existing logger, not console.log"`)
- places to stay out of (`"Don't change database migrations"`)
- where to look first (`"Reports code lives in app/Reports"`)

Keep it short. Long-lived project guidance belongs in the repo's `CLAUDE.md`,
which Claude Code reads anyway, for agents and for everyone else.

## Notes

- swat reads `.swat.json` from your **main clone**, not from the agent's
  worktree. Changes take effect for the next agent you start.
- `swat doctor` and `swat repos` show which of your clones have a `.swat.json`.
- The file is ignored if it isn't valid JSON. Comments aren't allowed in
  `.swat.json`, to keep it readable by other tools.
