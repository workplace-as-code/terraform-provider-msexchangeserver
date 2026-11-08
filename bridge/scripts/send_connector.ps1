# send_connector.ps1 - CRUD for send connectors (New/Get/Set/Remove-SendConnector).
#
# Same parameters as object.ps1. Address spaces are exchanged without the
# address type and the default cost: "contoso.com" for smtp:contoso.com;1,
# "*;100" for smtp:*;100.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [ValidateSet("SendConnector")] [string]$Noun = "SendConnector",
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

# ConvertTo-BridgeAddressSpace turns "smtp:contoso.com;1" into "contoso.com"
# and "smtp:*;100" into "*;100".
function ConvertTo-BridgeAddressSpace {
    param([string]$Value)
    $space = $Value -replace '^(?i)smtp:', ''
    if ($space -match '^(.*);1$') { return $Matches[1] }
    $space
}

function Read-BridgeConnector {
    $connector = Invoke-BridgeRetry { Get-SendConnector -Identity $Identity }
    $out = [ordered]@{ Name = [string]$connector.Name }
    foreach ($p in $props) {
        $value = Get-BridgeProp $connector $p
        $out[$p] = if ($p -eq "AddressSpaces") {
            ,([string[]]@(ConvertTo-BridgeStrings $value | ForEach-Object { ConvertTo-BridgeAddressSpace $_ }))
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
            if (-not $splat.ContainsKey("AddressSpaces")) { throw "address_spaces is required to create a send connector" }
            # -Force: New-SendConnector asks to confirm some settings, which
            # fails in a non-interactive session.
            Invoke-BridgeRetry -Create { New-SendConnector -Name $Identity -Usage Custom -Force @splat | Out-Null }
            Read-BridgeConnector
        }
        "Update" {
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { Set-SendConnector -Identity $Identity -Force @splat | Out-Null } }
            Read-BridgeConnector
        }
        "Delete" {
            try {
                Remove-SendConnector -Identity $Identity @BridgeNoConfirm
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("SendConnector $Action failed: $($_.Exception.Message)")
    exit 1
}
