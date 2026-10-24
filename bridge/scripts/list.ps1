# list.ps1 — Lists the import IDs of the existing objects of one type, for the
# list resources (terraform query).
#
#   -Noun      cmdlet noun, e.g. "TransportRule": Get-<noun> returns the objects
#   -Property  property holding the import ID (Identity or Name)
#
# Output: JSON array of strings.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidatePattern('^[A-Za-z]+$')] [string]$Noun,
    [ValidateSet("Identity", "Name")] [string]$Property = "Identity"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

try {
    $cmdlet = Get-Command "Get-$Noun"
    # Some Get cmdlets return the first 1000 objects only by default.
    $size = if ($cmdlet.Parameters.ContainsKey("ResultSize")) { @{ ResultSize = "Unlimited" } } else { @{} }
    # Some Get cmdlets report "couldn't be found" when there is no object.
    $objects = try { @(Invoke-BridgeRetry { & "Get-$Noun" @size }) } catch {
        if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
        @()
    }
    $ids = @(foreach ($object in $objects) { [string](Get-BridgeProp $object $Property "") })
    ConvertTo-Json -Compress -InputObject ([string[]]@($ids | Where-Object { $_ }))
} catch {
    [Console]::Error.WriteLine("Listing $Noun failed: $($_.Exception.Message)")
    exit 1
}
