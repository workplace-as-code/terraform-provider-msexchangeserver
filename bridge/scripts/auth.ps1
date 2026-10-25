# auth.ps1 — Exchange Server authentication
# Dot-sourced by all bridge/scripts scripts.
#
# Runs in Windows PowerShell 5.1 (powershell.exe): the Exchange Server remote
# PowerShell endpoint uses a serialization format that PowerShell 7 remoting
# does not support.
#
# The role (EXCHANGE_BRIDGE_ROLE, set by the Go client) picks the connection:
#   Mailbox  Exchange remote PowerShell (http://<server>/PowerShell/) with
#            Kerberos: the machine must be joined to the domain and Server must
#            be the FQDN registered in Active Directory. The account needs the
#            Organization Management role group (or equivalent).
#   Edge     Edge Transport servers are outside the domain and have no remote
#            PowerShell endpoint: they are managed with the Exchange snap-in.
#            Without Server, Terraform runs on the Edge server and loads the
#            snap-in locally (current Windows account, a local administrator).
#            With Server, a WinRM session (Negotiate) to the Edge server with a
#            local administrator account loads the snap-in and imports it.

param(
    [string]$Server   = "",
    [string]$Username = "",
    [string]$Password = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Never prompt. The Exchange cmdlets are proxies created by Import-PSSession:
# they only forward the parameters passed explicitly, so a Confirm default in
# PSDefaultParameterValues would never reach the server, which would then
# prompt. Scripts splat @BridgeNoConfirm on the cmdlets that confirm
# (Remove-*, Enable-*, Disable-*).
$BridgeNoConfirm = @{ Confirm = $false }

$BridgeRole = if ($env:EXCHANGE_BRIDGE_ROLE) { $env:EXCHANGE_BRIDGE_ROLE } else { "Mailbox" }

# Exchange answers in the language of the session. The scripts recognize errors
# by their English text ("couldn't be found", ...): on a French machine a missing
# object would be reported as a failure. Ask for English, whatever the OS: here
# for the local snap-in (Edge), and in the session options of the remote
# sessions below, which otherwise inherit the culture of this machine.
[System.Globalization.CultureInfo]::CurrentCulture   = [System.Globalization.CultureInfo]::GetCultureInfo("en-US")
[System.Globalization.CultureInfo]::CurrentUICulture = [System.Globalization.CultureInfo]::GetCultureInfo("en-US")

# The connection runs in a child scope. This file is dot-sourced into every
# bridge script: a variable assigned here would overwrite the script's own
# parameter of the same name, without regard to case ($name, set to the snap-in
# name, once replaced the -Name of transport_rule.ps1). Only $BridgeNoConfirm
# and $BridgeRole above are meant for the scripts.
& {
    param([string]$Server, [string]$Username, [string]$Password, [string]$Role)

    # Secrets are passed by the Go client through environment variables, never on the
    # command line (command lines are visible to every local user and are logged by
    # process-creation auditing). Explicit parameters still win, for manual testing.
    if (-not $Password -and $env:EXCHANGE_BRIDGE_PASSWORD) { $Password = $env:EXCHANGE_BRIDGE_PASSWORD }

    # Windows PowerShell 5.1 adds a Count property to arrays through its type data,
    # so ConvertTo-Json writes some arrays as {"value":[...],"Count":n}.
    if (Get-TypeData System.Array) { Remove-TypeData System.Array }

    # Skip if Exchange cmdlets are already imported in this session. With the
    # persistent bridge host, one PowerShell process serves many requests, so the
    # session opened by the first request is reused by all the following ones.
    if (Get-Command Get-TransportRule -ErrorAction SilentlyContinue) { return }

    # The Exchange snap-in name changed across versions: take the registered one.
    $snapinFilter = { $_.Name -like "Microsoft.Exchange.Management.PowerShell.*" }

    if ($Role -eq "Edge" -and -not $Server) {
        $snapin = Get-PSSnapin -Registered | Where-Object $snapinFilter | Select-Object -First 1
        if ($null -eq $snapin) {
            throw "The Exchange management tools are not installed on this machine. With role = `"Edge`" and no server, Terraform must run on the Edge Transport server; set server to manage it remotely."
        }
        # Snap-ins are loaded for the whole session, not only this script's scope.
        Add-PSSnapin $snapin.Name
        return
    }

    if (-not $Server)   { throw "server is required" }
    if (-not $Username) { throw "username is required" }

    $securePassword = ConvertTo-SecureString $Password -AsPlainText -Force
    $cred           = New-Object System.Management.Automation.PSCredential($Username, $securePassword)

    if ($Role -eq "Edge") {
        try {
            # The Edge server is in a workgroup: Negotiate (NTLM) with a local
            # account. Over HTTP, WinRM on this machine must trust the server
            # (TrustedHosts); the traffic is still encrypted by Negotiate.
            $edgeOpts = New-PSSessionOption -Culture en-US -UICulture en-US
            $session = New-PSSession -ComputerName $Server -Credential $cred -Authentication Negotiate -SessionOption $edgeOpts
            $name = Invoke-Command -Session $session -ScriptBlock {
                param($Filter)
                $s = Get-PSSnapin -Registered | Where-Object ([scriptblock]::Create($Filter)) | Select-Object -First 1
                if ($s) { Add-PSSnapin $s.Name; $s.Name }
            } -ArgumentList $snapinFilter.ToString()
            if (-not $name) { throw "the Exchange management tools are not installed on $Server" }
            Import-Module (Import-PSSession $session -Module $name -DisableNameChecking -AllowClobber) -Global -DisableNameChecking | Out-Null
        } catch {
            throw "Failed to connect to the Edge Transport server ($Server): $_"
        }
        return
    }

    # SkipCA/CNCheck allows self-signed certs common on internal Exchange servers
    $sessionOpts = New-PSSessionOption -SkipCACheck -SkipCNCheck -SkipRevocationCheck -Culture en-US -UICulture en-US

    try {
        $session = New-PSSession `
            -ConfigurationName Microsoft.Exchange `
            -ConnectionUri "http://$Server/PowerShell/" `
            -Authentication Kerberos `
            -Credential $cred `
            -SessionOption $sessionOpts

        # Import-PSSession alone imports into the calling script's scope, which
        # disappears when the script ends. -Global keeps the cmdlets available to
        # later requests served by the same persistent bridge host process.
        Import-Module (Import-PSSession $session -DisableNameChecking -AllowClobber) -Global -DisableNameChecking | Out-Null
    } catch {
        throw "Failed to connect to Exchange Server ($Server): $_"
    }
} $Server $Username $Password $BridgeRole
