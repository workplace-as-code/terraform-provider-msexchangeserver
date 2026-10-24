# email_address_policy.ps1 — CRUD for Exchange email address policies
# Cmdlet noun: EmailAddressPolicy

param(


[string]$Server = "",
    [string]$Username = "", [string]$Password = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity     = "",
    [string]$Name         = "",
    # Settings: only the ones set in the Terraform configuration are sent.
    [string]$EnabledEmailAddressTemplates,
    [string]$Priority,
    [string]$IncludedRecipients,
    [string]$ConditionalDepartment,
    [string]$ConditionalCompany,
    # "true": apply the policy to the existing recipients (Update-EmailAddressPolicy).
    # The provider only sends it on create, or on an update that changes a setting.
    [string]$ApplyToRecipients = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

$settings = ConvertTo-BridgeSplat $PSBoundParameters -Int Priority -Csv EnabledEmailAddressTemplates `
    -String IncludedRecipients, ConditionalDepartment, ConditionalCompany
function ConvertTo-PolicyJson { param($pol)
    @{
        Identity                      = [string]$pol.Identity
        Name                          = [string]$pol.Name
        EnabledEmailAddressTemplates  = @(if ($pol.EnabledEmailAddressTemplates) { ConvertTo-BridgeStrings $pol.EnabledEmailAddressTemplates })
        # The default policy always comes last: its priority is "Lowest".
        Priority                      = if ([string]$pol.Priority -eq "Lowest") { 0 } else { [int][string]$pol.Priority }
        IncludedRecipients            = [string](Get-BridgeProp $pol "IncludedRecipients" "")
        ConditionalDepartment         = [string]((@(Get-BridgeProp $pol "ConditionalDepartment") | Where-Object { $_ }) -join ",")
        ConditionalCompany            = [string]((@(Get-BridgeProp $pol "ConditionalCompany") | Where-Object { $_ }) -join ",")
    } | ConvertTo-Json -Compress
}

function Get-PolicyParams {
    param([bool]$IsCreate = $false)
    $p = @{} + $settings
    # A new policy cannot be created with an empty list.
    if ($IsCreate -and $p.ContainsKey("EnabledEmailAddressTemplates") -and $null -eq $p["EnabledEmailAddressTemplates"]) {
        $p.Remove("EnabledEmailAddressTemplates")
    }
    # A new on-premises policy needs a recipient filter.
    if ($IsCreate -and -not $p["IncludedRecipients"]) { $p["IncludedRecipients"] = "AllRecipients" }
    $p
}

switch ($Action) {
    "Get" {
        try { ConvertTo-PolicyJson (Get-EmailAddressPolicy -Identity $Identity -ErrorAction Stop) }
        catch { $m = $_.Exception.Message; if ($m -match "couldn't be found|not found|ObjectNotFound") { exit 0 }; Write-Error $m; exit 1 }
    }
    "Create" {
        $p = Get-PolicyParams -IsCreate $true
        $p["Name"] = $Name
        New-EmailAddressPolicy @p | Out-Null
        if ($ApplyToRecipients -eq "true") { Update-EmailAddressPolicy -Identity $Name | Out-Null }
        ConvertTo-PolicyJson (Get-EmailAddressPolicy -Identity $Name -ErrorAction Stop)
    }
    "Update" {
        $p = Get-PolicyParams
        if ($p.Count -gt 0) { Set-EmailAddressPolicy -Identity $Identity @p | Out-Null }
        # Update-EmailAddressPolicy applies the policy to the existing recipients.
        if ($ApplyToRecipients -eq "true") { Update-EmailAddressPolicy -Identity $Identity | Out-Null }
        ConvertTo-PolicyJson (Get-EmailAddressPolicy -Identity $Identity -ErrorAction Stop)
    }
    "Delete" { Remove-EmailAddressPolicy -Identity $Identity @BridgeNoConfirm; Write-Output "{}" }
}
