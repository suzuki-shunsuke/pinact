---
name: pinact
description: |
  Read pinact's documentation with `pinact docs list` and `pinact docs show <name>` before answering.
  pinact is a CLI that pins GitHub Actions and reusable workflows to full commit SHAs, updates them, and verifies their version comments.
  Use for any question about pinact, `PINACT_*` environment variables, `pinact run` / `init` / `migrate` / `token`, the configuration file `.pinact.yaml`, pinact's exit codes, or errors from pinact.
  Use it too when a workflow fails a pinact check in CI, or when an action isn't pinned and it isn't clear why.
---

Don't answer from your training knowledge about pinact itself - its commands, flags,
environment variables, configuration, or what its errors mean. Run `pinact docs list` to
list the documentation, then `pinact docs show <name>` to read the relevant topics before
answering questions about pinact or troubleshooting its errors. `pinact docs show` prints
the whole topic; read it through before concluding. You don't need to re-read a topic you
already read in this session.

`pinact run` edits the files it is given. When you only want to know whether a repository
is pinned, run `pinact run --check`, which reports without editing. Don't run
`pinact run --update` on the user's behalf unless they asked for an update: it changes
which version of an action a workflow uses.

A non-zero exit code is not necessarily pinact failing. 1 means something needs pinning,
2 means an action pinact can't fix by itself, and 3 means pinact itself failed. Read
`pinact docs show exit_codes` before calling a run broken, and follow it to the topic for
the specific case, such as `codes/005` for a SHA pinned without a version comment.

Read the source only after the documentation has failed to answer the question, and read
it at the version that is installed - a repository checkout is usually `main`, which can
be far ahead of the binary the user runs.

Organization-specific practice - which actions a repository is allowed to use, whether
updates go through Renovate, and so on - is outside what `pinact docs` covers, so answer
that from the user's own documentation and the conversation. When the two disagree about
how pinact itself behaves, `pinact docs` wins.

If `pinact docs` is rejected as an unknown command, the installed pinact is older than
v5.0.0. Tell the user to upgrade, and don't fall back to guessing.

If `pinact` is not installed at all, point them at the install guide:

https://github.com/suzuki-shunsuke/pinact/blob/main/INSTALL.md
