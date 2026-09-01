---
page_title: "truenas_ssh_credential Resource - terraform-provider-truenas"
subcategory: ""
description: |-
  Manages an SSH connection in the TrueNAS keychain: a remote host, the account to log in as, and the truenas_ssh_keypair to authenticate with. Replication tasks and other features reference a connection by its ID rather than carrying the credentials themselves.
---

# truenas_ssh_credential (Resource)

Manages an SSH connection in the TrueNAS keychain: a remote host, the account to log in as, and the `truenas_ssh_keypair` to authenticate with. Replication tasks and other features reference a connection by its ID rather than carrying the credentials themselves.

## Example Usage

```terraform
resource "tls_private_key" "backup" {
  algorithm = "ED25519"
}

resource "truenas_ssh_keypair" "backup" {
  name = "backup-host"

  private_key            = tls_private_key.backup.private_key_openssh
  private_key_wo_version = 1
}

# The host key is discovered when the connection is created, so the remote host
# must be reachable at apply time.
resource "truenas_ssh_credential" "backup" {
  name = "backup-host"

  host           = "backup.example.com"
  username       = "truenas_replication"
  private_key_id = truenas_ssh_keypair.backup.id
}
```

### Pinning a known host key

```terraform
resource "truenas_ssh_credential" "backup" {
  name = "backup-host"

  host           = "backup.example.com"
  port           = 2222
  username       = "truenas_replication"
  private_key_id = truenas_ssh_keypair.backup.id

  remote_host_key = file("${path.module}/backup-host.pub")
}
```

## Host key trust

`remote_host_key` is the SSH equivalent of a `known_hosts` entry: the public
host keys this connection will accept, one per line, without the leading host
field. TrueNAS stores the value verbatim.

Left unset, the provider asks TrueNAS to scan the host once, while the resource
is being created, and trusts whatever answers. That is trust on first use, and
it inherits the weakness of the pattern: nothing has established that the host
answering the scan is the intended one, so a machine-in-the-middle present at
create time is trusted permanently.

Two further consequences are worth stating plainly:

* The host must be reachable when the resource is created. An unreachable host
  fails the apply rather than producing a connection that cannot be used.
* The provider does not re-scan a host that has not changed. A host key that
  changes later — a rebuilt appliance, or an attack — is neither corrected nor
  reported as drift; the connection simply stops working, and `terraform plan`
  stays empty. Set `remote_host_key` explicitly and Terraform will report and
  correct a configuration that no longer matches the appliance.
* Changing `host` or `port` while `remote_host_key` is unset does scan afresh,
  so the stored key always belongs to the host recorded alongside it. That scan
  is a new trust-on-first-use event carrying the same implications as the first:
  the new host is trusted on whatever answers at that moment, and nothing
  verifies it is the intended machine. An unreachable new host fails the apply
  and the connection keeps its previous host key.

Set `remote_host_key` from a key obtained over a channel you already trust when
the connection matters. `ssh-keyscan` on a trusted network produces the format
this attribute takes.

## Key pairs

`private_key_id` takes the ID of a `truenas_ssh_keypair`. TrueNAS does not
validate the reference: an ID naming a credential that does not exist, or one
that names an SSH connection rather than a key pair, is accepted and stored, and
surfaces only when a replication task or other consumer tries to connect.

The public half of the key pair has to be in the remote account's
`authorized_keys` before the connection can authenticate. Nothing in this
resource installs it; the appliance is not contacted for authentication until
something uses the connection.

## Not implemented

TrueNAS also exposes `keychaincredential.setup_ssh_connection`, which generates
a key pair and — in its `SEMI-AUTOMATIC` mode — logs in to another TrueNAS
machine with a username and password to install the public key and register a
matching connection on both ends. It is deliberately not wrapped here:

* The key it generates exists only on the appliance, so nothing in the
  configuration describes it and every plan would want to replace it.
* Semi-automatic setup needs the remote machine's login password at apply time
  and mutates the *remote* appliance, neither of which Terraform can represent
  or reverse on destroy.

Install the public key on the remote host by whatever means suits it — a
`truenas_user` resource against a second TrueNAS, a configuration-management
run, or by hand — and declare the connection here.

## Import

SSH connections can be imported using the keychain credential ID:

```shell
terraform import truenas_ssh_credential.backup 13
```

Everything this resource manages is readable from the API, including
`remote_host_key`, so an import needs no follow-up apply. The secret this
connection depends on lives in the `truenas_ssh_keypair` it references, which
carries its own import caveats.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `host` (String) Hostname or IP address of the remote SSH server.
- `name` (String) Name that distinguishes this connection from others in the keychain.
- `private_key_id` (String) ID of the `truenas_ssh_keypair` this connection authenticates with. TrueNAS does not check that the ID names an existing key pair, so a wrong ID surfaces only when something tries to use the connection.

### Optional

- `connect_timeout` (Number) Seconds to wait for the remote host to accept a connection.
- `port` (Number) Port the remote SSH server listens on.
- `remote_host_key` (String) Public host keys the remote host is trusted to present, one per line, in `known_hosts` format without the leading host field. Leave it unset to have TrueNAS scan the host once, when the connection is created, and trust whatever answers — an unchanged host is not re-verified afterwards, so a host key that changes later is neither detected nor reported as drift. Changing `host` or `port` while this attribute is unset scans afresh, which is a new trust-on-first-use event with the same implications as the first: the new host is trusted on whatever answers at that moment, and nothing verifies it is the intended machine.
- `username` (String) Account to log in to the remote host as.

### Read-Only

- `id` (String) Keychain credential ID.
