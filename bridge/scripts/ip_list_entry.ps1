# ip_list_entry.ps1 - Entries of the IP allow and block lists of the connection
# filter agent (Add/Get/Remove-IPAllowListEntry, -IPBlockListEntry), on Edge
# Transport servers.
#
# Same parameters as object.ps1. An entry has no name: -Identity is its IP
# address or range, as Get-<noun> shows it in IPRange. There is no Set- cmdlet:
# the provider replaces an entry to change it.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("IPAllowListEntry","IPBlockListEntry")] [string]$Noun,
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

$splat = ConvertFrom-BridgeValues $Values

# Find-BridgeEntry returns the entry whose range is -Identity, or $null.
function Find-BridgeEntry {
    @(Invoke-BridgeRetry { & "Get-$Noun" }) | Where-Object { [string]$_.IPRange -eq $Identity } | Select-Object -First 1
}

# ConvertTo-BridgeUtc formats a date as RFC 3339 in UTC; "" when there is none.
function ConvertTo-BridgeUtc {
    param($Value)
    if (-not $Value) { return "" }
    ([datetime]$Value).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ", [cultureinfo]::InvariantCulture)
}

function ConvertTo-BridgeEntry {
    param($Entry)
    [ordered]@{
        Name           = [string]$Entry.IPRange
        Comment        = [string](Get-BridgeProp $Entry "Comment" "")
        ExpirationTime = ConvertTo-BridgeUtc (Get-BridgeProp $Entry "ExpirationTime")
    } | ConvertTo-Json -Compress
}

try {
    switch ($Action) {
        "Get" {
            $entry = Find-BridgeEntry
            if ($null -eq $entry) { exit 0 }
            ConvertTo-BridgeEntry $entry
        }
        "Create" {
            $add = @{}
            if ($Identity -match '[/-]') { $add.IPRange = $Identity } else { $add.IPAddress = $Identity }
            if ($splat.ContainsKey("Comment") -and $splat["Comment"]) { $add.Comment = $splat["Comment"] }
            if ($splat.ContainsKey("ExpirationTime") -and $splat["ExpirationTime"]) {
                $add.ExpirationTime = [DateTimeOffset]::Parse($splat["ExpirationTime"], [cultureinfo]::InvariantCulture).LocalDateTime
            }
            Invoke-BridgeRetry { & "Add-$Noun" @add | Out-Null }
            $entry = Wait-BridgeConsistent -Read { Find-BridgeEntry } -Check { param($e) $null -ne $e }
            if ($null -eq $entry) {
                throw "The $Noun '$Identity' was added but is not listed under this IPRange: give the range as Get-$Noun shows it."
            }
            ConvertTo-BridgeEntry $entry
        }
        "Update" {
            # Every setting replaces the entry: nothing to change in place.
            $entry = Find-BridgeEntry
            if ($null -eq $entry) { throw "$Noun '$Identity' couldn't be found." }
            ConvertTo-BridgeEntry $entry
        }
        "Delete" {
            $entry = Find-BridgeEntry
            if ($null -ne $entry) { & "Remove-$Noun" -Identity $entry.Identity @BridgeNoConfirm }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("$Noun $Action failed: $($_.Exception.Message)")
    exit 1
}
