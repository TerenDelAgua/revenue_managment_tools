# `.harness/` — TEREN Hotels agent team

This directory is the **project-side** definition of the agent team. It
travels with the repo (commit it to git). The **runtime** agents live
under `~/.mavis/agents/<name>/` and are registered with the daemon.

## What's in here

```
.harness/
├── agent.md                  # Orchestrator (this project). Routing brain.
├── docs/
│   ├── ownership.md          # Which rein owns which directory.
│   ├── architecture.md       # Clean Arch rules + service map.
│   └── testing.md            # Test ID matrix (BT/FT/IT/PF).
├── reins/                    # Mirror of the runtime team (for git history).
│   ├── teren-backend/agent.md
│   ├── teren-frontend/agent.md
│   ├── teren-db/agent.md
│   ├── teren-design/agent.md
│   ├── teren-inventory/agent.md
│   └── teren-qa/agent.md
└── .backups/                 # Old AGENTS.md (ITINERA) — kept for reference.
```

## What's the source of truth?

- **Runtime:** `mavis agent info <name>` — what the daemon actually uses.
- **Project doc:** `.harness/reins/<name>/agent.md` — what gets committed
  to git so the team is portable across machines.

When you change a rein's body:

1. Edit `.harness/reins/<name>/agent.md` (this is the canonical text).
2. Run `mavis agent update <name> --system-prompt "<body>"` to push it
   to the runtime agent. (Or copy the file body in by hand if you
   prefer editing at `~/.mavis/agents/<name>/agent.md`.)

The 6 reins are the same names as the 6 global agents — that
intentional: the orchestrator (in `agent.md`) routes by name.

## Roster

| Name | Role | Owns |
| --- | --- | --- |
| `teren-backend` | Go/Chi/pgx dev | `backend/internal/**`, `backend/cmd/**` |
| `teren-frontend` | SvelteKit 5 dev | `web/src/**`, `web/package.json` scripts |
| `teren-db` | PostgreSQL specialist | `backend/migrations/**`, `backend/seeds/**`, raw SQL |
| `teren-design` | Design system + a11y | `Docs/TEREN_DESIGN_SYSTEM.md`, visual review |
| `teren-inventory` | Domain specialist | Availability, blocks, booking lifecycle, RevPAR |
| `teren-qa` | Tests + verification | `web/tests/**`, `*_test.go`, perf budgets |

## Why no `.harness/reins` is daemon-mounted

`mavis harness mount` fails on Windows with `EPERM: fsync` (a known
issue when the harness dir lives on certain file systems). The reins
therefore live as project files (this folder) + global agents
(`~/.mavis/agents/`) — both kept in sync by hand. When `mavis harness
mount` works on your machine, the `.harness/reins/` files will be the
ones the daemon reads directly.
