# `.context/.tmp/` — markdown staging area

Gitignored scratch space for agents. Drop scratch/temp markdown here — PR or issue
bodies for `gh --body-file`, drafts, intermediate notes — instead of at the repo root
or in `.claude/`.

Everything in this folder is ignored except `.gitignore`, `.gitkeep`, and this `README.md`
(force-added so the folder + docs stay tracked). New scratch never shows up in
`git status` / GitHub Desktop. Delete after use anyway — the gitignore is the safety net,
not a licence to accumulate.
