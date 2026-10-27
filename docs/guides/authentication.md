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
- **An account** (its password in `username` / `password`, or the account Terraform runs as with [integrated authentication](#integrated-windows-authentication)) with the **Organization Management** role group, or equivalent, as `CONTOSO\svc-terraform` or `svc-terraform@contoso.local`. Exchange remote PowerShell only exposes the cmdlets that the account's management roles allow.

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

## Integrated Windows authentication

With `auth_method = "integrated"`, the provider connects with the **Windows identity of the process running Terraform**: no `username` and no `password` in the configuration, the CI variables or the job environment. The password of the account is typed once, in Windows, when the service or the scheduled task that runs Terraform is configured, or set there by the tool that rotates it (CyberArk, HashiCorp Vault).

```hcl
provider "msexchangeserver" {
  server      = "exch01.contoso.local"
  auth_method = "integrated"
}
```

Or `$env:EXCHANGE_AUTH_METHOD = "integrated"` with `EXCHANGE_SERVER`. Setting `username` or `password` (or `EXCHANGE_USERNAME` / `EXCHANGE_PASSWORD`) with `integrated` is an error: the provider never guesses which account you meant. Integrated authentication works with the `Mailbox` role only.

### Where Terraform runs

No interactive logon is needed: a **Windows service** or a **scheduled task** runs under its account, and its processes get that account's Kerberos tickets.

- **The account** is a classic Active Directory user (not a group managed service account: the Exchange management shell does not accept them), member of the Exchange role group the provider needs, as for the `password` method.
- **On the machine that runs Terraform**, joined to the domain, the account needs the **Log on as a service** right (or **Log on as a batch job** for a scheduled task). It never needs to be a local administrator.
- **A scheduled task** must use **Run whether user is logged on or not** with the password stored. With **Do not store password**, Windows runs the task without network credentials: the account gets no Kerberos ticket and the connection to Exchange fails.
- **A CI runner** is installed as a service under that account. For GitLab Runner:

  ```powershell
  .\gitlab-runner.exe install --user "CONTOSO\svc-tf-exch-write" --password "<typed once>" --working-directory C:\GitLab-Runner-write --config C:\GitLab-Runner-write\config.toml --service gitlab-runner-write
  ```

### One read account and one write account

The Exchange rights are those of the account the runner runs as. To keep `terraform plan` read-only, use **two runners** on the machine, each a service under its own account:

| Runner | Account | Exchange role group | Runs |
|---|---|---|---|
| tag `exchange-read` | `CONTOSO\svc-tf-exch-read` | View-Only Organization Management | `plan` on merge requests, scheduled drift detection |
| tag `exchange-write` | `CONTOSO\svc-tf-exch-write` | the provider's least-privilege role group | `apply` only, from the default branch |

Register the write runner as a **protected** runner, so that it only runs jobs of protected branches: a merge request cannot run code under the write account. The pipeline picks the runner by tag:

```yaml
variables:
  EXCHANGE_SERVER: exch01.contoso.local
  EXCHANGE_AUTH_METHOD: integrated

plan:
  tags: [exchange-read]
  script:
    - terraform plan "-out=plan.cache"
  artifacts:
    paths: [plan.cache]

apply:
  tags: [exchange-write]
  needs: [plan]
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
      when: manual
  script:
    # The plan reviewed on the read runner, passed as an artifact
    - terraform apply plan.cache
```

If the read account or the pipeline of a merge request is misused, the configuration can be read but not changed.

### Checking the account

From a PowerShell session running as the account (for example `runas /user:CONTOSO\svc-tf-exch-read powershell.exe` on a test machine), the same check as above without `-Credential`:

```powershell
whoami
$session = New-PSSession -ConfigurationName Microsoft.Exchange -ConnectionUri "http://exch01.contoso.local/PowerShell/" -Authentication Kerberos
Invoke-Command -Session $session -ScriptBlock { Get-AcceptedDomain }
Remove-PSSession $session
```

When the connection fails, the provider's error names the account it ran as: `Failed to connect to Exchange Server (exch01.contoso.local) as CONTOSO\svc-tf-exch-read: ...`. Check that the machine is joined to the domain, that `server` is the FQDN registered in Active Directory, that `klist` shows a ticket for the account, and that the account is a member of an Exchange role group.

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

With [integrated Windows authentication](#integrated-windows-authentication), there is no password to pass at all: the account's password stays in the Windows service configuration.

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
