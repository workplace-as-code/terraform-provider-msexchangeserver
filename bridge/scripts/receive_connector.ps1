# receive_connector.ps1 - CRUD for receive connectors
# (New/Get/Set/Remove-ReceiveConnector).
#
# Same parameters as object.ps1. -Identity is "<server>\<name>": the connector
# is created on that server. Flags (PermissionGroups, AuthMechanism) are
# exchanged as lists of names.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [ValidateSet("ReceiveConnector")] [string]$Noun = "ReceiveConnector",
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
$flags = @("PermissionGroups", "AuthMechanism")

function Read-BridgeConnector {
    $connector = Invoke-BridgeRetry { Get-ReceiveConnector -Identity $Identity }
    $out = [ordered]@{ Name = [string]$connector.Identity }
    foreach ($p in $props) {
        $value = Get-BridgeProp $connector $p
        $out[$p] = if ($flags -contains $p) {
            # A remote session returns the flags as "AnonymousUsers, ExchangeServers".
            ,([string[]]@("$value" -split ',\s*' | Where-Object { $_ -ne "" }))
        } else {
            ConvertTo-BridgeValue $value
        }
    }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgeConnector } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }
        "Create" {
            $parts = $Identity -split '\\', 2
            if ($parts.Count -ne 2 -or -not $parts[0] -or -not $parts[1]) {
                throw "name must be <server>\<name>, got '$Identity'"
            }
            if (-not $splat.ContainsKey("Bindings")) { throw "bindings is required to create a receive connector" }
            Invoke-BridgeRetry { New-ReceiveConnector -Name $parts[1] -Server $parts[0] -Usage Custom @splat | Out-Null }
            Read-BridgeConnector
        }
        "Update" {
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { Set-ReceiveConnector -Identity $Identity @splat | Out-Null } }
            Read-BridgeConnector
        }
        "Delete" {
            try {
                Remove-ReceiveConnector -Identity $Identity @BridgeNoConfirm
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("ReceiveConnector $Action failed: $($_.Exception.Message)")
    exit 1
}
