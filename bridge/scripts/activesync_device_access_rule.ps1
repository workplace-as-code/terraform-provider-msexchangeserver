# activesync_device_access_rule.ps1 - CRUD for one Exchange ActiveSync device
# access rule (New-ActiveSyncDeviceAccessRule, Get-ActiveSyncDeviceAccessRule,
# Set-ActiveSyncDeviceAccessRule, Remove-ActiveSyncDeviceAccessRule). A rule has no
# name of its own: it is found by its query string and characteristic, which
# never change. Only the access level can be updated.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [string]$QueryString,
    [Parameter(Mandatory)] [ValidateSet("DeviceType","DeviceModel","DeviceOS","UserAgent","XMSWLHeader")] [string]$Characteristic,
    [ValidateSet("", "Allow", "Block", "Quarantine")] [string]$AccessLevel = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

# Get-Rule_ returns the rule matching the query string and characteristic, or
# $null. Filtering the list avoids building the Exchange name, whose form is
# not documented.
function Get-Rule_ {
    @(Invoke-BridgeRetry { Get-ActiveSyncDeviceAccessRule }) |
        Where-Object { [string]$_.QueryString -eq $QueryString -and [string]$_.Characteristic -eq $Characteristic } |
        Select-Object -First 1
}

function ConvertTo-BridgeRule {
    param($Rule)
    [ordered]@{
        Name           = [string](Get-BridgeProp $Rule "Name" "")
        QueryString    = [string](Get-BridgeProp $Rule "QueryString" "")
        Characteristic = [string](Get-BridgeProp $Rule "Characteristic" "")
        AccessLevel    = [string](Get-BridgeProp $Rule "AccessLevel" "")
    } | ConvertTo-Json -Compress
}

try {
    switch ($Action) {
        "Get" {
            $rule = Get-Rule_
            if ($null -eq $rule) { exit 0 }
            ConvertTo-BridgeRule $rule
        }

        "Create" {
            if (-not $AccessLevel) { throw "access_level is required to create a rule" }
            Invoke-BridgeRetry -Create {
                New-ActiveSyncDeviceAccessRule -QueryString $QueryString -Characteristic $Characteristic -AccessLevel $AccessLevel | Out-Null
            }
            $rule = Wait-BridgeConsistent -Read { Get-Rule_ } -Check { param($r) $null -ne $r }
            if ($null -eq $rule) { throw "The rule '$QueryString ($Characteristic)' was created but cannot be read back." }
            ConvertTo-BridgeRule $rule
        }

        "Update" {
            $rule = Get-Rule_
            if ($null -eq $rule) { throw "The rule '$QueryString ($Characteristic)' couldn't be found." }
            if ($AccessLevel) { Invoke-BridgeRetry { Set-ActiveSyncDeviceAccessRule -Identity ([string]$rule.Identity) -AccessLevel $AccessLevel | Out-Null } }
            ConvertTo-BridgeRule (Wait-BridgeConsistent -Read { Get-Rule_ } -Check {
                param($r) -not $AccessLevel -or ($null -ne $r -and [string]$r.AccessLevel -eq $AccessLevel) })
        }

        "Delete" {
            $rule = Get-Rule_
            if ($null -ne $rule) { Invoke-BridgeRetry { Remove-ActiveSyncDeviceAccessRule -Identity ([string]$rule.Identity) @BridgeNoConfirm } }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("activesync_device_access_rule $Action failed: $($_.Exception.Message)")
    exit 1
}
