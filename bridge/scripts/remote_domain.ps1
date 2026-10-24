# remote_domains.ps1 — CRUD for Exchange remote domains
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity,
    [string]$Name,
    [string]$DomainName,
    # Settings: only the ones set in the Terraform configuration are sent.
    # Booleans arrive as "true"/"false".
    [string]$AutoReplyEnabled,
    [string]$AutoForwardEnabled,
    [string]$TrustedMailInboundEnabled,
    [string]$TrustedMailOutboundEnabled,
    [string]$TnefEnabled,                   # "" = inherit the organization default
    [string]$AllowedOofType
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password

. "$PSScriptRoot/common.ps1"

$settings = ConvertTo-BridgeSplat $PSBoundParameters -Bool AutoReplyEnabled, AutoForwardEnabled, TrustedMailInboundEnabled, TrustedMailOutboundEnabled -String AllowedOofType
# TnefEnabled is tri-state: "true", "false", or "" to inherit from the organization.
if ($PSBoundParameters.ContainsKey("TnefEnabled")) {
    $settings["TNEFEnabled"] = if ($TnefEnabled -eq "true") { $true } elseif ($TnefEnabled -eq "false") { $false } else { $null }
}

function ConvertTo-BridgeDomain {
    param($Domain)
    @{
        Identity                   = [string]$Domain.Identity
        Name                       = $Domain.Name
        DomainName                 = $Domain.DomainName.ToString()
        AutoReplyEnabled           = [bool]$Domain.AutoReplyEnabled
        AutoForwardEnabled         = [bool]$Domain.AutoForwardEnabled
        TrustedMailInboundEnabled  = [bool]$Domain.TrustedMailInboundEnabled
        TrustedMailOutboundEnabled = [bool]$Domain.TrustedMailOutboundEnabled
        TnefEnabled                = if ($null -ne $Domain.TNEFEnabled) { [bool]$Domain.TNEFEnabled } else { $null }
        AllowedOofType             = $Domain.AllowedOOFType.ToString()
    } | ConvertTo-Json -Compress
}

switch ($Action) {

    "Get" {
        try {
            $domain = Get-RemoteDomain -Identity $Identity -ErrorAction Stop
            ConvertTo-BridgeDomain -Domain $domain
        } catch {
            # Only "object not found" means the resource is gone; any other error (auth,
            # throttling, network) must fail the run instead of dropping it from state.
            if ($_.Exception.Message -match "couldn't be found|not found|ObjectNotFound") { exit 0 }
            [Console]::Error.WriteLine("ERROR in remote_domain Get: $($_.Exception.Message)")
            exit 1
        }
    }

    "Create" {
        # New-RemoteDomain only accepts Name and DomainName; other settings go via Set-RemoteDomain
        New-RemoteDomain -Name $Name -DomainName $DomainName | Out-Null
        if ($settings.Count -gt 0) { Set-RemoteDomain -Identity $Name @settings }

        $domain = Get-RemoteDomain -Identity $Name
        ConvertTo-BridgeDomain -Domain $domain
    }

    "Update" {
        if ($settings.Count -gt 0) { Set-RemoteDomain -Identity $Identity @settings }

        $domain = Get-RemoteDomain -Identity $Identity
        ConvertTo-BridgeDomain -Domain $domain
    }

    "Delete" {
        Remove-RemoteDomain -Identity $Identity @BridgeNoConfirm
        Write-Output "{}"
    }
}
