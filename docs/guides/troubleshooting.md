---
page_title: "Troubleshooting"
subcategory: "Guides"
---

# Troubleshooting

The provider runs Windows PowerShell scripts that open a remote PowerShell session on an Exchange server. When something fails, find the layer first: installing the provider, Windows PowerShell, the Kerberos connection, the account's roles, or the Exchange operation itself. The sections below follow that order. The commands are written for Windows PowerShell, on the machine that runs Terraform.

## Read the Error

When an Exchange cmdlet fails, the error ends with the PowerShell message, unchanged:

```text
PowerShell script transport_rule.ps1 failed: <message of the failing cmdlet>
```

The script name tells which resource failed: each resource has its own script (`transport_rule.ps1`, `remote_domain.ps1`...), the generic one is `object.ps1`, and `list.ps1` lists objects for `terraform query`. The message after `failed:` comes from Exchange or from PowerShell: search for it as is. For Kerberos errors, access denied and missing cmdlets, the provider adds a paragraph of advice after the message.

## Provider Installation

**`no available releases match the given constraints`, although the version exists on the registry.** A local copy of the provider hides the registry. Terraform looks for providers in local plugin directories first (on Windows `%APPDATA%\terraform.d\plugins` and `%APPDATA%\HashiCorp\Terraform\plugins`). When it finds a provider there, it installs that provider only from there. Remove the `workplace-as-code` folder from these directories, or declare an explicit `provider_installation` block in the CLI configuration.

**`Provider development overrides are in effect`.** The CLI configuration (`%APPDATA%\terraform.rc`, or the file named by `TF_CLI_CONFIG_FILE`) has a `dev_overrides` entry for the provider: Terraform runs a local build instead of the published version. Remove the entry to use the registry.

**Checksum errors on another machine** (`does not match any of the checksums recorded in the dependency lock file`). The lock file only lists the platforms it was created on. When the configuration is also edited or validated on Linux or macOS, record every platform:

```powershell
terraform providers lock -platform=windows_amd64 -platform=linux_amd64 -platform=darwin_arm64
```

**No Internet access.** The registry sends Terraform to GitHub to download the release files. Exchange servers and the machines next to them often have no Internet access: serve the provider from a network mirror (`terraform providers mirror`, or a repository manager such as Artifactory) and point the CLI configuration at it.

## Windows PowerShell

**`the provider needs Windows PowerShell 5.1 (powershell.exe), which needs a Windows host`.** Terraform runs on Linux or macOS. Exchange Server remote PowerShell only works from Windows PowerShell 5.1: run Terraform on a Windows machine joined to the domain. PowerShell 7 is not used and does not need to be installed.

**`PowerShell bridge host failed to start`, `did not start within`, or `exited while running`.** The provider keeps a few PowerShell processes open and sends them requests. To get the error of a single call, closer to the failing cmdlet, run once with one process per call:

```powershell
$env:EXCHANGE_BRIDGE_MODE = "oneshot"
terraform plan
Remove-Item Env:EXCHANGE_BRIDGE_MODE
```

The provider runs its scripts with `-ExecutionPolicy Bypass` for its own processes only: the execution policy of the machine does not need to change.

## Connection

Connection errors start with `Failed to connect to Exchange Server (<server>):`. Test the connection outside Terraform first, with the commands of the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/authentication).

**Kerberos errors** (the message mentions `WinRM` and `Kerberos`). Kerberos needs, all at once:

- a machine **joined** to the Active Directory domain of the Exchange organization, or to a trusted domain;
- `server` set to the **FQDN of an Exchange server as registered in Active Directory**: an IP address, a short name, or a DNS alias or load balancer name fails;
- access to a domain controller (port 88) and to the Exchange server (port 80);
- clocks within five minutes of the domain controllers.

**`Access is denied`.** The account cannot open the session. Check its password, its membership of the **Organization Management** role group (or an equivalent), and that remote PowerShell is enabled for it:

```powershell
Get-User svc-terraform | Format-List RemotePowerShellEnabled
```

## Edge Transport Servers

**`The Exchange management tools are not installed on this machine`.** With `role = "Edge"` and no `server`, Terraform must run on the Edge Transport server itself, which has the Exchange snap-in.

**`Failed to connect to the Edge Transport server (<server>):`.** With `server` set, the provider opens a WinRM session (port 5985, Negotiate authentication) with a local administrator of the Edge server. Check that:

- WinRM is enabled on the Edge server (`Enable-PSRemoting`) and the perimeter firewall allows port 5985;
- the Terraform machine trusts the Edge server, which is outside the domain: `Set-Item WSMan:\localhost\Client\TrustedHosts -Value edge01.perimeter.contoso.com -Concatenate`;
- `username` is a local administrator of the Edge server, for example `EDGE01\Administrator`.

**`the Exchange management tools are not installed on <server>`.** The WinRM session works, but the server named in `server` is not an Edge Transport server.

**Exchange refuses changes to accepted or remote domains on Edge.** While the Edge server is subscribed to EdgeSync, these domains are copied from the organization: manage them with the provider block of the organization instead.

## Permissions

**`The term 'New-TransportRule' is not recognized` (or any other Exchange cmdlet).** Exchange remote PowerShell only contains the cmdlets that the account's management roles allow: a missing role looks like a missing cmdlet. Give the account the role that contains the cmdlet, or the Organization Management role group. The provider adds this advice to the error.

## Messages in English

Exchange answers in the language of the PowerShell session. The provider recognizes some errors, such as a missing object, by their English text, so it opens its sessions in English (`en-US`), whatever the display language of the machine that runs Terraform. Exchange errors are therefore always in English: search for them as they are.

## Timeouts and Load

**`PowerShell script <name> timed out after 10m (set EXCHANGE_SCRIPT_TIMEOUT to raise the limit)`.** One call took longer than the limit. The first call of a run also opens the remote session and imports the Exchange cmdlets. Raise the limit, for example `$env:EXCHANGE_SCRIPT_TIMEOUT = "20m"`.

**Many objects, or a busy Exchange server.** Each PowerShell process holds a remote PowerShell session, and Exchange throttling policies limit the number of sessions per account. `EXCHANGE_MAX_CONCURRENCY` (default `4`) limits the number of processes: lower it, for example to `2`, to reduce the load.

## Debug Logging

```powershell
$env:TF_LOG_PROVIDER = "DEBUG"
$env:TF_LOG_PATH = "terraform.log"
terraform plan
```

`TF_LOG_PROVIDER` limits the log to the providers; `TF_LOG=DEBUG` adds Terraform itself. The log shows which resource and which operation failed, in order. The PowerShell error is already in the error message. Remove the variables afterwards (`Remove-Item Env:TF_LOG_PROVIDER, Env:TF_LOG_PATH`).

The log can contain values from your configuration: read it before sharing it. The password is never written to the log or put on a command line: the provider passes it to PowerShell through the process environment.

## Run a Bridge Script by Hand

Running the script that failed, outside Terraform, shows the full PowerShell error and lets you try a fix.

1. Find the scripts. The provider writes them, at each start, to `%LOCALAPPDATA%\terraform-provider-msexchangeserver\<hash>\`, a folder named after their content hash. They are also in the repository, under `bridge/scripts`, at the tag of your provider version.

2. In Windows PowerShell, from that folder, connect and call the script with the same parameters as the provider:

   ```powershell
   Set-ExecutionPolicy -Scope Process Bypass
   $cred = Get-Credential "CONTOSO\svc-terraform"
   $auth = @{
       Server   = "exch01.contoso.local"
       Username = $cred.UserName
       Password = $cred.GetNetworkCredential().Password
   }

   # Lists the remote domains, as terraform query does
   .\list.ps1 @auth -Noun RemoteDomain

   # Reads one remote domain, as terraform plan does
   .\remote_domain.ps1 @auth -Action Get -Identity "Contoso-Partner"
   ```

   For an Edge Transport server, set `$env:EXCHANGE_BRIDGE_ROLE = "Edge"` first. `-Noun` is the noun of the Exchange cmdlets of the resource, listed in its documentation (`Get-RemoteDomain` → `RemoteDomain`). A read prints one line of JSON; an empty output means that the object does not exist.

3. To test a change to a script, copy the folder, edit the copy and point the provider at it with `$env:EXCHANGE_BRIDGE_DIR = "C:\path\to\the\copy"`. This is for script development only: remove the variable afterwards.

## Report a Problem

If none of this helps, [open an issue](https://github.com/workplace-as-code/terraform-provider-msexchangeserver/issues/new/choose) with the provider, Terraform and Exchange Server versions (`Get-ExchangeServer | Format-Table Name, ServerRole, AdminDisplayVersion`), the resource configuration and the full error message. Remove server names, domains and account names first.
