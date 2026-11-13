# journal_rule.ps1 - CRUD for journal rules (New-JournalRule, Get-JournalRule,
# Set-JournalRule, Remove-JournalRule), with the same parameters as object.ps1.
# Journal rule specifics:
#   - Enabled is set with New-JournalRule -Enabled, then with Enable-JournalRule
#     and Disable-JournalRule, and read from the Enabled property: a rule
#     created disabled never journals a message
#   - an empty Recipient clears it: every recipient is journaled

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [ValidateSet("JournalRule")] [string]$Noun = "JournalRule",
    [Parameter(Mandatory)] [string]$Identity,
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

$splat = ConvertFrom-BridgeValues $Values
$props = @(ConvertFrom-BridgeJson $Properties)

# New-JournalRule and Set-JournalRule refuse "" for an address: $null clears it.
foreach ($k in @($splat.Keys)) { if ($splat[$k] -is [string] -and $splat[$k] -eq "") { $splat[$k] = $null } }
$enabled = $null
if ($splat.ContainsKey("Enabled")) { $enabled = [bool]$splat["Enabled"]; $splat.Remove("Enabled") }

function Read-BridgeJournalRule {
    $rule = Invoke-BridgeRetry { Get-JournalRule -Identity $Identity }
    $out = [ordered]@{ Name = [string]$rule.Name }
    foreach ($p in $props) { $out[$p] = ConvertTo-BridgeValue (Get-BridgeProp $rule $p) }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try {
                Read-BridgeJournalRule
            } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            $new = @{}
            foreach ($k in $splat.Keys) { if ($null -ne $splat[$k]) { $new[$k] = $splat[$k] } }
            if ($null -ne $enabled) { $new["Enabled"] = $enabled }
            Invoke-BridgeRetry -Create { New-JournalRule -Name $Identity @new | Out-Null }
            Read-BridgeJournalRule
        }

        "Update" {
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { Set-JournalRule -Identity $Identity @splat | Out-Null } }
            if ($enabled -eq $true)  { Invoke-BridgeRetry { Enable-JournalRule -Identity $Identity @BridgeNoConfirm | Out-Null } }
            if ($enabled -eq $false) { Invoke-BridgeRetry { Disable-JournalRule -Identity $Identity @BridgeNoConfirm | Out-Null } }
            Read-BridgeJournalRule
        }

        "Delete" {
            try {
                Remove-JournalRule -Identity $Identity @BridgeNoConfirm
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("JournalRule $Action failed: $($_.Exception.Message)")
    exit 1
}
