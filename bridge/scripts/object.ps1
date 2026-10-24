# object.ps1 — Generic CRUD for Exchange objects managed with plain
# New/Get/Set/Remove cmdlets, and for organization-wide settings (Get/Set only).
#
# The Go resource describes the object with a cmdlet noun and the list of
# properties it manages:
#   -Noun        cmdlet noun, e.g. "RetentionPolicyTag"
#   -Identity    object name; empty for a setting read with Get-<noun> alone
#   -Values      base64 JSON object: cmdlet parameter -> value, only the
#                attributes set in the Terraform configuration
#   -Properties  base64 JSON array: properties returned to Terraform

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidatePattern('^[A-Za-z]+$')] [string]$Noun,
    [string]$Identity   = "",
    [string]$Values     = "",
    [string]$Properties = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

$splat = ConvertFrom-BridgeValues $Values
$props = @(ConvertFrom-BridgeJson $Properties)
$identityArgs = if ($Identity) { @{ Identity = $Identity } } else { @{} }

function Read-BridgeObject {
    param([string]$Name = $Identity)
    $args_ = if ($Name) { @{ Identity = $Name } } else { @{} }
    $object = Invoke-BridgeRetry { @(& "Get-$Noun" @args_) | Select-Object -First 1 }
    if ($null -eq $object) { throw "$Noun '$Name' couldn't be found." }
    $out = [ordered]@{ Name = [string](Get-BridgeProp $object "Name" "") }
    foreach ($p in $props) { $out[$p] = ConvertTo-BridgeValue (Get-BridgeProp $object $p) }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try {
                Read-BridgeObject
            } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            Invoke-BridgeRetry { & "New-$Noun" -Name $Identity @splat | Out-Null }
            Read-BridgeObject
        }

        "Update" {
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { & "Set-$Noun" @identityArgs @splat | Out-Null } }
            # An empty list arrives as $null. Most cmdlets clear the property,
            # but some ignore it (Set-ExternalInOutlook -AllowList $null keeps
            # the list): remove the remaining values explicitly.
            $emptied = @($splat.Keys | Where-Object { $null -eq $splat[$_] })
            if ($emptied.Count -gt 0) {
                $current = Invoke-BridgeRetry { @(& "Get-$Noun" @identityArgs) | Select-Object -First 1 }
                $remove = @{}
                foreach ($k in $emptied) {
                    $remaining = @(Get-BridgeProp $current $k | Where-Object { $null -ne $_ } | ForEach-Object { $_.ToString() })
                    if ($remaining.Count -gt 0) { $remove[$k] = @{ Remove = [string[]]$remaining } }
                }
                if ($remove.Count -gt 0) {
                    Invoke-BridgeRetry { & "Set-$Noun" @identityArgs @remove | Out-Null }
                    # The removal can take a few seconds to show up in reads.
                    Wait-BridgeConsistent -Read { @(& "Get-$Noun" @identityArgs) | Select-Object -First 1 } -Check {
                        param($o)
                        -not @($remove.Keys | Where-Object { @(Get-BridgeProp $o $_ | Where-Object { $null -ne $_ }).Count -gt 0 })
                    } | Out-Null
                }
            }
            Read-BridgeObject
        }

        "Delete" {
            try {
                & "Remove-$Noun" -Identity $Identity @BridgeNoConfirm
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("$Noun $Action failed: $($_.Exception.Message)")
    exit 1
}
