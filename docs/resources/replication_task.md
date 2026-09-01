---
page_title: "truenas_replication_task Resource - terraform-provider-truenas"
subcategory: ""
description: |-
  Manages a ZFS replication task that pushes snapshots to a remote host over SSH. Snapshots are not created by this resource: bind the task to a naming schema that matches snapshots produced elsewhere, for example by a periodic snapshot task.
---

# truenas_replication_task (Resource)

Manages a ZFS replication task that pushes snapshots to a remote host over SSH. Snapshots are not created by this resource: bind the task to a naming schema that matches snapshots produced elsewhere, for example by a periodic snapshot task.

## Example Usage

### Nightly Push to a Backup Host

```terraform
# Push nightly snapshots of an archive dataset to a backup host over SSH.
#
# The SSH connection is a keychain credential of type SSH_CREDENTIALS. The
# provider does not manage keychain credentials, so create it in the TrueNAS
# UI under Credentials > Backup Credentials and reference its numeric ID.
variable "backup_ssh_credential" {
  description = "ID of the SSH_CREDENTIALS keychain credential for the backup host."
  type        = number
}

resource "truenas_dataset" "archive" {
  pool = "tank"
  path = "archive"
}

resource "truenas_replication_task" "archive_to_backup" {
  name            = "archive to backup"
  ssh_credentials = var.backup_ssh_credential

  source_datasets = ["${truenas_dataset.archive.pool}/${truenas_dataset.archive.path}"]
  target_dataset  = "backup/archive"
  recursive       = true

  # Replicate the snapshots a periodic snapshot task produces.
  also_include_naming_schema = ["auto-%Y-%m-%d_%H-%M"]

  retention_policy = "CUSTOM"
  lifetime_value   = 2
  lifetime_unit    = "WEEK"

  compression = "LZ4"

  schedule {
    minute = "0"
    hour   = "3"
  }
}
```

### Manual Task

Omit the `schedule` block for a task that only runs when it is triggered by hand.
`auto` reads back as `false`, and the task keeps its configuration until it is run.

```terraform
resource "truenas_replication_task" "seed" {
  name            = "initial seed"
  ssh_credentials = var.backup_ssh_credential

  source_datasets = ["tank/archive"]
  target_dataset  = "backup/archive"

  also_include_naming_schema = ["auto-%Y-%m-%d_%H-%M"]

  retention_policy = "SOURCE"
}
```

## Snapshots Are Not Created Here

A replication task transfers snapshots; it never takes them. `also_include_naming_schema`
matches snapshots that already exist on the source, so pair the task with whatever
creates them — a periodic snapshot task using the same naming schema, for example.
A task whose schema matches nothing runs successfully and transfers nothing.

## SSH Credentials

`ssh_credentials` is the numeric ID of a keychain credential of type `SSH_CREDENTIALS`.
The provider does not manage keychain credentials: create the credential in the TrueNAS
UI under **Credentials > Backup Credentials > SSH Connections**, or through
`keychaincredential.create`, and pass its ID here.

Replication runs `zfs` on the target host. The credential normally authenticates as
`root`; for any other account, set `sudo = true` and configure passwordless sudo on
the target.

## Validation

These cross-field rules are checked during `terraform plan`, so a configuration the
API would reject never reaches the server:

- `ssh_credentials` is required for the `SSH` transport.
- `lifetime_value` and `lifetime_unit` are required by the `CUSTOM` retention policy,
  and rejected by every other policy.
- `exclude` requires `recursive`.

## Unimplemented by Scope Decision

This resource covers one shape deliberately: an SSH push from local datasets to a
target on a remote SSH credential. The `replication.*` API supports considerably more,
and the following is **not** implemented. Each item maps to an optional attribute that
can be added without changing anything documented above, so a configuration written
today keeps working when the gaps are filled.

| Area | Omitted | Notes |
| --- | --- | --- |
| Direction | `PULL` | `direction` accepts `PUSH` only. Pull replication needs `naming_schema` instead of `also_include_naming_schema`, and a different set of validation rules. |
| Transport | `LOCAL`, `SSH+NETCAT` | `transport` accepts `SSH` only. `SSH+NETCAT` additionally needs `netcat_active_side`, `netcat_active_side_listen_address`, `netcat_active_side_port_min`, `netcat_active_side_port_max` and `netcat_passive_side_connect_address`; `LOCAL` forbids `ssh_credentials`. |
| Snapshot selection | `periodic_snapshot_tasks`, `name_regex` | Bind the task to a naming schema instead. Binding periodic snapshot tasks also changes how `schedule` behaves, replacing it with `restrict_schedule`. |
| Scheduling | `restrict_schedule`, `only_matching_schedule` | Only relevant to tasks bound to periodic snapshot tasks. |
| Retention | `lifetimes` | Multi-tier retention, where each tier has its own cron and lifetime. Single-tier `CUSTOM` retention is supported. |
| Replication mode | `replicate` | Full ZFS replication of the dataset, rather than of matching snapshots. |
| Properties | `properties`, `properties_exclude`, `properties_override` | The server's defaults apply: dataset properties are sent along with snapshots. |
| Encryption | `encryption`, `encryption_inherit`, `encryption_key`, `encryption_key_format`, `encryption_key_location` | Encrypting the replicated datasets at the destination. |
| Stream tuning | `large_block`, `embed`, `compressed` | The server's defaults apply. `compression` and `speed_limit`, which shape the SSH stream itself, are supported. |
| Failure handling | `hold_pending_snapshots` | Whether retention may delete source snapshots after a failed run. |
| Actions | `replication.run`, `replication.run_onetime`, `replication.restore` | Running a task is an operation, not state Terraform can hold. Trigger runs from the UI, the CLI, or `midclt`. |

Importing a task that uses one of the modes above is refused rather than silently
adopted: the import fails with an error naming the offending value, so an out-of-scope
task is never brought under management and then rewritten by the next apply.

A task already under management that is flipped to an out-of-scope mode on the server
is treated differently: the refresh emits a warning and keeps the task in state with
the server's values, so it stays plannable and, above all, destroyable. Applying the
configuration unchanged will propose rewriting the task back into scope; `terraform
destroy` or `terraform state rm` removes it instead.

Fields the resource does not send are left at the server's defaults, and are not read
back into state — changing one outside Terraform produces no diff.

## Import

Replication tasks can be imported using the numeric task ID:

```shell
terraform import truenas_replication_task.example 1
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `also_include_naming_schema` (List of String) `strftime` patterns identifying the snapshots to replicate, for example `auto-%Y-%m-%d_%H-%M`.
- `name` (String) Name of the replication task.
- `retention_policy` (String) How snapshots are deleted on the target. `SOURCE` deletes snapshots that no longer exist on the source, `CUSTOM` deletes snapshots older than `lifetime_value`/`lifetime_unit`, and `NONE` never deletes.
- `source_datasets` (List of String) Datasets to replicate snapshots from, without a leading `/mnt`.
- `target_dataset` (String) Dataset on the target host to put snapshots into, without a leading `/mnt`.

### Optional

- `allow_from_scratch` (Boolean) Destroy every snapshot on the target and replicate from scratch when no target snapshot matches the source.
- `compression` (String) Compress the SSH stream. Only valid for the `SSH` transport.
- `direction` (String) Direction of the transfer. Only `PUSH` is supported.
- `enabled` (Boolean) Enable the replication task. A disabled task neither runs on its schedule nor can be triggered manually.
- `exclude` (List of String) Child datasets to exclude from replication. Requires `recursive`.
- `lifetime_unit` (String) Unit `lifetime_value` counts. Required by, and only valid with, the `CUSTOM` retention policy.
- `lifetime_value` (Number) How many `lifetime_unit`s to keep snapshots for. Required by, and only valid with, the `CUSTOM` retention policy.
- `logging_level` (String) Verbosity of the task's log.
- `readonly` (String) How the target datasets' `readonly` property is handled. `SET` marks them read-only after each run, `REQUIRE` refuses to run unless they already are, and `IGNORE` leaves the property alone.
- `recursive` (Boolean) Replicate child datasets of every source dataset.
- `retries` (Number) Number of attempts before a run is considered failed.
- `schedule` (Block, Optional) When the task runs. Omit the block for a task that only runs when triggered manually. (see [below for nested schema](#nestedblock--schedule))
- `speed_limit` (Number) Limit the SSH stream to this many bytes per second. Only valid for the `SSH` transport.
- `ssh_credentials` (Number) ID of the `SSH_CREDENTIALS` keychain credential identifying the target host. Required for the `SSH` transport.
- `sudo` (Boolean) Run `zfs` through passwordless sudo on the target host. Needed when the credential's user is not root.
- `transport` (String) Method of snapshot transfer. Only `SSH` is supported.

### Read-Only

- `auto` (Boolean) Whether the task runs by itself. Derived from `schedule`: a task with a schedule runs automatically, one without it only runs when triggered manually.
- `id` (String) Replication task ID.
- `state` (String) Last known run state of the task, such as `PENDING`, `RUNNING`, `FINISHED` or `ERROR`.

<a id="nestedblock--schedule"></a>
### Nested Schema for `schedule`

Optional:

- `begin` (String) Earliest time of day the task may start, in `HH:MM`.
- `dom` (String) Day of month (1-31 or cron expression).
- `dow` (String) Day of week (1-7, Monday to Sunday, or cron expression).
- `end` (String) Latest time of day the task may start, in `HH:MM`.
- `hour` (String) Hour (0-23 or cron expression).
- `minute` (String) Minute (0-59 or cron expression).
- `month` (String) Month (1-12 or cron expression).
