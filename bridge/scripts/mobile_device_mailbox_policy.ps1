# mobile_device_mailbox_policy.ps1 — CRUD for Exchange Mobile Device Mailbox Policies
# Cmdlet noun: MobileDeviceMailboxPolicy

param(


[string]$Server = "",
    [string]$Username = "", [string]$Password = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity                        = "",
    [string]$Name                            = "",
    [string]$IsDefault,
    [string]$PasswordEnabled,
    [string]$AlphanumericPasswordRequired,
    [string]$PasswordRecoveryEnabled,
    [string]$DeviceEncryptionEnabled,
    [string]$AllowSimplePassword,
    [string]$MaxPasswordFailedAttempts,
    [string]$MinPasswordLength,
    [string]$MaxInactivityTimeLock,
    [string]$RequireDeviceEncryption,
    [string]$AllowStorageCard,
    [string]$AllowCamera,
    [string]$AllowWiFi,
    [string]$AllowTextMessaging,
    [string]$AllowBluetooth,
    [string]$AllowBrowser
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

# ConvertFrom-UnlimitedInt reads an Unlimited<int> value: "Unlimited" is 0.
function ConvertFrom-UnlimitedInt { param($Value, [int]$Default)
    if ($null -eq $Value) { return $Default }
    $s = [string]$Value
    if ($s -eq "Unlimited") { return 0 }
    [int]$s
}

function ConvertTo-PolicyJson { param($pol)
    @{
        Identity                     = [string]$pol.Identity
        Name                         = [string]$pol.Name
        IsDefault                    = [bool]$pol.IsDefault
        PasswordEnabled              = [bool]$pol.PasswordEnabled
        AlphanumericPasswordRequired = [bool]$pol.AlphanumericPasswordRequired
        PasswordRecoveryEnabled      = [bool]$pol.PasswordRecoveryEnabled
        DeviceEncryptionEnabled      = [bool]$pol.DeviceEncryptionEnabled
        AllowSimplePassword          = [bool]$pol.AllowSimplePassword
        MaxPasswordFailedAttempts    = ConvertFrom-UnlimitedInt $pol.MaxPasswordFailedAttempts 8
        MinPasswordLength            = if ($null -ne $pol.MinPasswordLength)         { [int]$pol.MinPasswordLength }         else { 4 }
        MaxInactivityTimeLock        = if ($pol.MaxInactivityTimeLock)               { [string]$pol.MaxInactivityTimeLock }  else { "00:15:00" }
        RequireDeviceEncryption      = [bool]$pol.RequireDeviceEncryption
        AllowStorageCard             = [bool]$pol.AllowStorageCard
        AllowCamera                  = [bool]$pol.AllowCamera
        AllowWiFi                    = [bool]$pol.AllowWiFi
        AllowTextMessaging           = [bool]$pol.AllowTextMessaging
        AllowBluetooth               = [string]$pol.AllowBluetooth
        AllowBrowser                 = [bool]$pol.AllowBrowser
    } | ConvertTo-Json -Compress
}

# Only the settings set in the Terraform configuration are sent.
$settings = ConvertTo-BridgeSplat $PSBoundParameters `
    -Bool IsDefault, PasswordEnabled, AlphanumericPasswordRequired, PasswordRecoveryEnabled, DeviceEncryptionEnabled, AllowSimplePassword, RequireDeviceEncryption, AllowStorageCard, AllowCamera, AllowWiFi, AllowTextMessaging, AllowBrowser `
    -Int MinPasswordLength -String MaxInactivityTimeLock, AllowBluetooth
# 0 failed attempts means unlimited.
if ($PSBoundParameters.ContainsKey("MaxPasswordFailedAttempts")) {
    $settings["MaxPasswordFailedAttempts"] = if ([int]$MaxPasswordFailedAttempts -eq 0) { "Unlimited" } else { [int]$MaxPasswordFailedAttempts }
}

switch ($Action) {
    "Get" {
        try { ConvertTo-PolicyJson (Get-MobileDeviceMailboxPolicy -Identity $Identity -ErrorAction Stop) }
        catch { $m = $_.Exception.Message; if ($m -match "couldn't be found|not found|ObjectNotFound") { exit 0 }; Write-Error $m; exit 1 }
    }
    "Create" {
        New-MobileDeviceMailboxPolicy -Name $Name @settings | Out-Null
        ConvertTo-PolicyJson (Get-MobileDeviceMailboxPolicy -Identity $Name -ErrorAction Stop)
    }
    "Update" {
        if ($settings.Count -gt 0) { Set-MobileDeviceMailboxPolicy -Identity $Identity @settings | Out-Null }
        ConvertTo-PolicyJson (Get-MobileDeviceMailboxPolicy -Identity $Identity -ErrorAction Stop)
    }
    "Delete" { Remove-MobileDeviceMailboxPolicy -Identity $Identity @BridgeNoConfirm; Write-Output "{}" }
}
