---
page_title: "Authentication Guide"
subcategory: "Guides"
---

# Authentication Guide

The provider opens a remote PowerShell session on an Exchange server, at `http://<server>/PowerShell/`, with **Kerberos** authentication: the default configuration of the Exchange PowerShell virtual directory. Kerberos encrypts the session even over HTTP.

## Requirements

- **A Windows machine joined to the Active Directory domain** of the Exchange organization (or to a trusted domain), to run Terraform. It uses Windows PowerShell 5.1, which is built into Windows: PowerShell 7 does not work with Exchange Server remote PowerShell, and no module needs to be installed, since the cmdlets come from the Exchange server.
- **The FQDN of an Exchange server** in `server`, as registered in Active Directory, for example `exch01.contoso.local`. Kerberos fails with an IP address, a short name, or a DNS alias or load balancer name (unless that name has an alternate service account).
- **Network access** from that machine to a domain controller (Kerberos, port 88) and to the Exchange server (HTTP, port 80).
- **An account** with the **Organization Management** role group, or equivalent, as `CONTOSO\svc-terraform` or `svc-terraform@contoso.local`. Exchange remote PowerShell only exposes the cmdlets that the account's management roles allow.

Check the connection from the Terraform machine before running Terraform:

```powershell
$cred    = Get-Credential "CONTOSO\svc-terraform"
$session = New-PSSession -ConfigurationName Microsoft.Exchange -ConnectionUri "http://exch01.contoso.local/PowerShell/" -Authentication Kerberos -Credential $cred
Invoke-Command -Session $session -ScriptBlock { Get-AcceptedDomain }
Remove-PSSession $session
```

## Configuration

```hcl
provider "msexchangeserver" {
  server   = "exch01.contoso.local"
  username = var.username
  password = var.password
}
```

Or via environment variables, in PowerShell:

```powershell
$env:EXCHANGE_SERVER   = "exch01.contoso.local"
$env:EXCHANGE_USERNAME = "CONTOSO\svc-terraform"
$env:EXCHANGE_PASSWORD = "your-password"
```

A value set in the provider block takes precedence over the environment.

---

## Edge Transport servers

An Edge Transport server sits in the perimeter network, outside the domain, and has no remote PowerShell endpoint: it is managed with the Exchange management snap-in, and its configuration is its own (with the accepted and remote domains copied by EdgeSync when it is subscribed). Manage it with a separate provider block whose `role` is `Edge`, usually with an alias next to the block of the organization:

```hcl
provider "msexchangeserver" {
  server   = "exch01.contoso.local"
  username = var.username
  password = var.password
}

provider "msexchangeserver" {
  alias    = "edge01"
  role     = "Edge"
  server   = "edge01.perimeter.contoso.com"
  username = "EDGE01\\Administrator"
  password = var.edge_password
}

resource "msexchangeserver_transport_rule" "edge_quarantine" {
  provider = msexchangeserver.edge01
  # ...
}
```

The provider connects to an Edge Transport server in one of two ways:

- **Remotely**, with `server` set: a WinRM session (PowerShell remoting, port 5985) with **Negotiate** authentication and a **local administrator** of the Edge server, which loads the Exchange snap-in there. WinRM must be enabled on the Edge server (`Enable-PSRemoting`), the perimeter firewall must allow the connection, and, since the Edge server is not in the domain, the Terraform machine must trust it: `Set-Item WSMan:\localhost\Client\TrustedHosts -Value edge01.perimeter.contoso.com -Concatenate`.
- **Locally**, without `server`: Terraform runs on the Edge server itself, as a local administrator, and loads the snap-in in its own PowerShell process. `username` and `password` are not used.

With the `Edge` role, only the resources that exist on an Edge Transport server can be planned: `msexchangeserver_transport_rule` (where the `Quarantine` action is only available on Edge), `msexchangeserver_accepted_domain`, `msexchangeserver_remote_domain` and `msexchangeserver_transport_config`. While the server is subscribed to EdgeSync, Exchange refuses changes to the accepted and remote domains on the Edge server: manage them on the organization instead.

---

## Storing Secrets Safely

Never commit the password to version control. Pass it through `EXCHANGE_PASSWORD` or a `sensitive` variable, filled from a secret store at run time: HashiCorp Vault, Azure Key Vault, or the secrets of your CI system (GitHub Actions, Azure DevOps, GitLab).

Use a dedicated service account for Terraform rather than a personal administrator account, so that its rights and its activity in the Exchange admin audit log stay separate.

---

## Runtime tuning

These optional environment variables control how the provider runs PowerShell:

| Variable | Default | Purpose |
|---|---|---|
| `EXCHANGE_SCRIPT_TIMEOUT` | `10m` | Maximum duration of one bridge script run, as a Go duration. |
| `EXCHANGE_MAX_CONCURRENCY` | `4` | Maximum number of PowerShell processes running at once. Lower it to reduce the load on the Exchange server. |
| `EXCHANGE_BRIDGE_DIR` | unset | Run the bridge scripts from this folder instead of the copy embedded in the provider. For script development only. |
| `EXCHANGE_BRIDGE_MODE` | `persistent` | `persistent` keeps up to `EXCHANGE_MAX_CONCURRENCY` PowerShell processes alive and reuses their Exchange connection. `oneshot` starts a new process, and a new connection, for every operation. Use it only for troubleshooting. |
| `EXCHANGE_WORKER_MAX_REQUESTS` | `200` | Number of operations a persistent PowerShell process serves before it is replaced. |

In persistent mode, a process that hits an error, a timeout or a crash is discarded and the next operation starts a fresh one. The processes disconnect from Exchange when Terraform finishes.

---

## Troubleshooting

Kerberos, WinRM, access and role errors are covered in the [Troubleshooting guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/troubleshooting).
