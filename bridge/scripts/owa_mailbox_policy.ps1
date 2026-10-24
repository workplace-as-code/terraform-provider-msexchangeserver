# owa_mailbox_policy.ps1 — CRUD for Exchange OWA mailbox policies
# Cmdlet noun: OwaMailboxPolicy → Get/New/Set/Remove-OwaMailboxPolicy
# No associated rule — policies are assigned directly to mailboxes via Set-CASMailbox.
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity = "",
    [string]$Name = "",
    # Settings: only the ones set in the Terraform configuration are sent.
    # Booleans arrive as "true"/"false".
    [string]$InstantMessagingEnabled,
    [string]$ActiveSyncIntegrationEnabled,
    [string]$ClassicAttachmentsEnabled,
    [string]$PublicFoldersEnabled,
    [string]$RecoverDeletedItemsEnabled,
    [string]$RulesEnabled,
    [string]$SignaturesEnabled,
    [string]$ThemeSelectionEnabled,
    [string]$WacViewingOnPrivateComputersEnabled,
    [string]$WacViewingOnPublicComputersEnabled,
    # String enums
    [string]$WebPartsFrameOptionsType,
    [string]$OutboundCharset,
    # Lists passed as comma-separated strings from Go
    [string]$BlockedFileTypes,
    [string]$BlockedMimeTypes
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

# Convert string booleans from Go to native PowerShell booleans

# Convert CSV strings from Go into string arrays (empty string → empty array)
# Exchange requires file type extensions with a leading dot (e.g. ".exe"), so we normalize here.
# Build the parameter hashtable for Set-OwaMailboxPolicy
$settings = ConvertTo-BridgeSplat $PSBoundParameters `
    -Bool InstantMessagingEnabled, ActiveSyncIntegrationEnabled, ClassicAttachmentsEnabled, PublicFoldersEnabled, RecoverDeletedItemsEnabled, RulesEnabled, SignaturesEnabled, ThemeSelectionEnabled, WacViewingOnPrivateComputersEnabled, WacViewingOnPublicComputersEnabled `
    -String WebPartsFrameOptionsType, OutboundCharset -Csv BlockedFileTypes, BlockedMimeTypes
# Exchange stores file extensions with their leading dot.
if ($null -ne $settings["BlockedFileTypes"]) {
    $settings["BlockedFileTypes"] = [string[]]@($settings["BlockedFileTypes"] | ForEach-Object { if ($_ -match '^\.') { $_ } else { ".$_" } })
}

# Read-WrittenPolicy reads the policy until it shows the blocked file types
# just written (see Wait-BridgeConsistent).
function Read-WrittenPolicy {
    param([string]$PolicyIdentity)
    if (-not $settings.ContainsKey("BlockedFileTypes")) { return Get-OwaMailboxPolicy -Identity $PolicyIdentity }
    $wanted = @($settings["BlockedFileTypes"] | Where-Object { $_ } | Sort-Object)
    Wait-BridgeConsistent -Read { Get-OwaMailboxPolicy -Identity $PolicyIdentity } -Check {
        param($p)
        $actual = @(@($p.BlockedFileTypes) | Where-Object { $_ } | ForEach-Object { $_.ToString() } | Sort-Object)
        ($actual -join ",") -eq ($wanted -join ",")
    }
}

# Map policy object to the JSON schema returned to Terraform
function ConvertTo-BridgePolicy {
    param($Policy)
    @{
        Identity                            = [string]$Policy.Identity
        Name                                = $Policy.Name
        InstantMessagingEnabled             = $Policy.InstantMessagingEnabled
        ActiveSyncIntegrationEnabled        = $Policy.ActiveSyncIntegrationEnabled
        AttachmentsEnabled                  = if ($Policy.PSObject.Properties['ClassicAttachmentsEnabled']) { $Policy.ClassicAttachmentsEnabled } else { $true }
        PublicFoldersEnabled                = $Policy.PublicFoldersEnabled
        RecoverDeletedItemsEnabled          = $Policy.RecoverDeletedItemsEnabled
        RulesEnabled                        = $Policy.RulesEnabled
        SignaturesEnabled                   = $Policy.SignaturesEnabled
        ThemeSelectionEnabled               = $Policy.ThemeSelectionEnabled
        WacViewingOnPrivateComputersEnabled = $Policy.WacViewingOnPrivateComputersEnabled
        WacViewingOnPublicComputersEnabled  = $Policy.WacViewingOnPublicComputersEnabled
        WebPartsFrameOptionsType            = [string]$Policy.WebPartsFrameOptionsType
        OutboundCharset                     = [string]$Policy.OutboundCharset
        BlockedFileTypes                    = @(if ($Policy.BlockedFileTypes) { ConvertTo-BridgeStrings $Policy.BlockedFileTypes | ForEach-Object { $_ -replace '^\.' } })
        BlockedMimeTypes                    = @(if ($Policy.BlockedMimeTypes) { ConvertTo-BridgeStrings $Policy.BlockedMimeTypes })
    } | ConvertTo-Json -Compress
}

switch ($Action) {

    "Get" {
        try {
            $policy = Get-OwaMailboxPolicy -Identity $Identity -ErrorAction Stop
            ConvertTo-BridgePolicy -Policy $policy
        } catch {
            # Only "object not found" means the resource is gone; any other error (auth,
            # throttling, network) must fail the run instead of dropping it from state.
            if ($_.Exception.Message -match "couldn't be found|not found|ObjectNotFound") { exit 0 }
            [Console]::Error.WriteLine("ERROR in owa_mailbox_policy Get: $($_.Exception.Message)")
            exit 1
        }
    }

    "Create" {
        # New-OwaMailboxPolicy only accepts -Name; all settings applied via Set afterward
        New-OwaMailboxPolicy -Name $Name | Out-Null

        if ($settings.Count -gt 0) { Set-OwaMailboxPolicy -Identity $Name @settings }

        ConvertTo-BridgePolicy -Policy (Read-WrittenPolicy $Name)
    }

    "Update" {
        if ($settings.Count -gt 0) { Set-OwaMailboxPolicy -Identity $Identity @settings }

        ConvertTo-BridgePolicy -Policy (Read-WrittenPolicy $Identity)
    }

    "Delete" {
        Remove-OwaMailboxPolicy -Identity $Identity @BridgeNoConfirm
        Write-Output "{}"
    }
}
