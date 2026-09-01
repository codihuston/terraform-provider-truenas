# Terraform Provider TrueNAS

This repository contains a Terraform provider for TrueNAS SCALE and Community editions. It uses the `midclt` command to communicate with the TrueNAS API.

## Development workflow

1. Development tasks are conducted using `mise`. Run `mise tasks` to see what tasks are available.
2. To explore the TrueNAS API:
   - `mr api-docs api_methods` - Browse available API methods
   - `mr api-docs api_methods_{namespace}` - Browse methods in a namespace (e.g., `api_methods_cloudsync`)
   - `mr api-docs {doc}` - View formatted documentation (uses lynx, add `-r` for raw RST)
   - `mr midclt-method {method}` - Get JSON schema for a specific method (better for implementation)

### Acceptance tests

Acceptance tests live beside the unit tests in `internal/resources` (package
`resources_test`) and are named `TestAcc*`. They are gated on `TF_ACC=1` and
skip otherwise, so `mise run test` never touches a real server. Shared helpers
(provider factory, precheck, live API client) are in
`internal/resources/acceptance_test.go`.

Run them with `mise run test-acc` after exporting:

| Variable | Purpose |
| --- | --- |
| `TRUENAS_HOST` | Target hostname or IP |
| `TRUENAS_API_KEY` | API key for the WebSocket transport |
| `TRUENAS_API_USER` | User the API key belongs to (default `root`) |
| `TRUENAS_SSH_PRIVATE_KEY` | SSH private key **contents** (the provider requires an SSH block even in WebSocket mode) |
| `TRUENAS_SSH_USER` | SSH user the key belongs to (default `root`) |
| `TRUENAS_SSH_HOST_KEY_FINGERPRINT` | Host key fingerprint — must match the algorithm the client negotiates (usually ECDSA, not ED25519); get all of them with `ssh-keyscan <host> \| ssh-keygen -lf -` |
| `TRUENAS_ACC_POOL` | Pool to create test datasets in (default `tank`) |

Tests must assert server-side state through the API (not just Terraform state)
and clean up after themselves via `CheckDestroy`.

The SSH connection is not optional even for the WebSocket transport: SSH detects
the TrueNAS version and backs the file operations, and the client runs `midclt`
under `sudo`, so the SSH account needs passwordless sudo as well as an authorised
key. To drive a resource by hand instead, build the provider and point Terraform
at it with a `dev_overrides` CLI configuration.

### Design and implementation plans

When asked to write an implementation plan, the context should include the current code coverage from `mise run coverage`. In the verification tasks, verify that the code coverage has either improved or maintained with the baseline. 

### Finishing a development branch

When finishing a development branch:

1. Make sure coverage is equal to or better than the baseline.
2. Clean up the docs/plans/ folder and commit.

## Ethos

- Always write idiomatic terraform.
- Strive for 100% code coverage where possible.

## Git Rules

- Never use `git -C` flag. Always `cd` to the working directory first or work from the current directory.

## Worktrees

- Feature development uses git worktrees in `.worktrees/` (already in .gitignore).
- Copy local Claude settings to new worktrees

### Using `tea` from a worktree

`tea` cannot auto-detect the repository when running from a worktree. Specify all parameters explicitly:

```bash
# Find your login name (use the NAME column)
tea login list

# Create PR with explicit parameters
tea pr create \
  --login <login-name> \
  --repo sh/terraform-provider-truenas \
  --head <branch-name> \
  --base main \
  --title "..." \
  --description "..."
```

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
