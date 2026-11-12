# server_settings.ps1 - Get/Set for the settings that every Exchange server has
# once: its Transport service (Get-TransportService, Set-TransportService), its
# Front End Transport service (Get-FrontendTransportService,
# Set-FrontendTransportService), and its POP3 and IMAP4 settings
# (Get-PopSettings, Set-PopSettings, Get-ImapSettings, Set-ImapSettings). They
# are adopted, never created or removed.
#
# Same parameters as object.ps1. -Identity is the server name or FQDN; it is
# returned as the name, in the form it was given. The POP3 and IMAP4 cmdlets
# take it as -Server, the others as -Identity.
#
# Every server has these settings: a server that cannot be found is an error,
# never a missing object, or the resource would silently leave the state. On a
# Mailbox connection, an Edge Transport server is refused: what Get-
# TransportService returns for it is the copy EdgeSync keeps in Active
# Directory, not the settings of the Edge server.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)]
    [ValidateSet("TransportService","FrontendTransportService","PopSettings","ImapSettings")]
    [string]$Noun,
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
# An empty string clears a setting: the cmdlets take $null, not an empty string.
foreach ($k in @($splat.Keys)) {
    if ($splat[$k] -is [string] -and $splat[$k] -eq "") { $splat[$k] = $null }
}
$targetArgs = if ($Noun -in "PopSettings", "ImapSettings") { @{ Server = $Identity } } else { @{ Identity = $Identity } }
# Test-BridgeNotFound never takes this message for a missing object.
$unknownServer = "Exchange server '$Identity' was not found"

# Get-BridgeTarget returns the settings of the server -Identity names.
function Get-BridgeTarget {
    if ($BridgeRole -ne "Edge") {
        $exchangeServer = try { Invoke-BridgeRetry { Get-ExchangeServer -Identity $Identity } } catch {
            if (Test-BridgeNotFound $_.Exception.Message) { throw "${unknownServer}: $($_.Exception.Message)" }
            throw
        }
        if ([bool](Get-BridgeProp $exchangeServer "IsEdgeServer" $false)) {
            throw "'$Identity' is an Edge Transport server: manage its settings with a provider whose role is Edge."
        }
    }
    $object = try { @(Invoke-BridgeRetry { & "Get-$Noun" @targetArgs }) | Select-Object -First 1 } catch {
        if (Test-BridgeNotFound $_.Exception.Message) { throw "${unknownServer}: $($_.Exception.Message)" }
        throw
    }
    if ($null -eq $object) { throw "$unknownServer." }
    $object
}

function Read-BridgeTarget {
    $object = Get-BridgeTarget
    $out = [ordered]@{ Name = $Identity }
    foreach ($p in $props) { $out[$p] = ConvertTo-BridgeValue (Get-BridgeProp $object $p) }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" { Read-BridgeTarget }
        { $_ -in "Create", "Update" } {
            if ($splat.Count -gt 0) {
                Get-BridgeTarget | Out-Null
                Invoke-BridgeRetry { & "Set-$Noun" @targetArgs @splat | Out-Null }
            }
            Read-BridgeTarget
        }
        "Delete" { "{}" }
    }
} catch {
    [Console]::Error.WriteLine("$Noun $Action failed: $($_.Exception.Message)")
    exit 1
}
