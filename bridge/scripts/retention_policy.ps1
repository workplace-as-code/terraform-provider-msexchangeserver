# retention_policy.ps1 — CRUD for Exchange Retention Policies
# Cmdlet noun: RetentionPolicy
# Note: RetentionPolicyTag management is separate — this resource manages the policy container.

param(


[string]$Server = "",
    [string]$Username = "", [string]$Password = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity             = "",
    [string]$Name                 = "",
    # Settings: only the ones set in the Terraform configuration are sent.
    [string]$RetentionPolicyTagLinks
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

# Only the settings set in the Terraform configuration are sent.
$settings = ConvertTo-BridgeSplat $PSBoundParameters -Csv RetentionPolicyTagLinks



function ConvertTo-PolicyJson { param($pol)
    @{
        Identity                      = [string]$pol.Identity
        Name                          = [string]$pol.Name
        RetentionPolicyTagLinks       = @(if ($pol.RetentionPolicyTagLinks) { ConvertTo-BridgeStrings $pol.RetentionPolicyTagLinks })
        IsDefault                     = [bool]$pol.IsDefault
        IsDefaultArbitrationMailbox   = [bool]$pol.IsDefaultArbitrationMailbox
    } | ConvertTo-Json -Compress
}

switch ($Action) {
    "Get" {
        try { ConvertTo-PolicyJson (Get-RetentionPolicy -Identity $Identity -ErrorAction Stop) }
        catch { $m = $_.Exception.Message; if ($m -match "couldn't be found|not found|ObjectNotFound") { exit 0 }; Write-Error $m; exit 1 }
    }
    "Create" {
        $p = @{ Name = $Name }
        foreach ($k in $settings.Keys) { if ($null -ne $settings[$k]) { $p[$k] = $settings[$k] } }
        New-RetentionPolicy @p | Out-Null
        ConvertTo-PolicyJson (Get-RetentionPolicy -Identity $Name -ErrorAction Stop)
    }
    "Update" {
        if ($settings.Count -gt 0) { Set-RetentionPolicy -Identity $Identity @settings | Out-Null }
        ConvertTo-PolicyJson (Get-RetentionPolicy -Identity $Identity -ErrorAction Stop)
    }
    "Delete" { Remove-RetentionPolicy -Identity $Identity @BridgeNoConfirm; Write-Output "{}" }
}
