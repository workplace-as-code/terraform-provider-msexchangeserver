# ip_block_list_provider.ps1 - IP block list providers of the connection filter
# agent (Add/Get/Set/Remove-IPBlockListProvider), on Edge Transport servers.
# Same parameters as object.ps1: only creation differs, with Add- instead of New-.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [ValidateSet("IPBlockListProvider")] [string]$Noun = "IPBlockListProvider",
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

function Read-BridgeProvider {
    $provider = Invoke-BridgeRetry { Get-IPBlockListProvider -Identity $Identity }
    $out = [ordered]@{ Name = [string]$provider.Name }
    foreach ($p in $props) { $out[$p] = ConvertTo-BridgeValue (Get-BridgeProp $provider $p) }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgeProvider } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }
        "Create" {
            if (-not $splat.ContainsKey("LookupDomain")) { throw "lookup_domain is required to create a provider" }
            Invoke-BridgeRetry { Add-IPBlockListProvider -Name $Identity @splat | Out-Null }
            Read-BridgeProvider
        }
        "Update" {
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { Set-IPBlockListProvider -Identity $Identity @splat | Out-Null } }
            Read-BridgeProvider
        }
        "Delete" {
            try {
                Remove-IPBlockListProvider -Identity $Identity @BridgeNoConfirm
            } catch {
                if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("IPBlockListProvider $Action failed: $($_.Exception.Message)")
    exit 1
}
