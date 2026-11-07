# virtual_directory.ps1 - Get/Set for the virtual directories of a server
# (Get/Set-OwaVirtualDirectory, ...) and for its Client Access services
# (Get/Set-ClientAccessService), and for Outlook Anywhere (the /Rpc directory,
# Get/Set-OutlookAnywhere). They exist on every server: they are adopted,
# never created or removed.
#
# Same parameters as object.ps1. -Identity is the server name or FQDN, for the
# virtual directory of the Default Web Site, or the identity of the virtual
# directory ("EX01\owa (Default Web Site)"). It is returned as the name, in the
# form it was given. A directory is found in Active Directory
# (-ADPropertiesOnly), then read in full, from IIS too: the authentication
# settings live there, and -ADPropertiesOnly returns False for them. A server
# whose IIS does not answer is an error, never AD-only values.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)]
    [ValidateSet("OwaVirtualDirectory","EcpVirtualDirectory","WebServicesVirtualDirectory","ActiveSyncVirtualDirectory",
        "OabVirtualDirectory","MapiVirtualDirectory","OutlookAnywhere","ClientAccessService")]
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
# An empty URL clears it: the cmdlets take $null, not an empty string.
foreach ($k in @($splat.Keys)) {
    if ("$($splat[$k])" -eq "") { $splat[$k] = $null }
}

# Set when the directory exists but IIS could not be read: that error must
# never pass for "not found" (its text may say "The network path was not found").
$IisReadFailed = $false

# Get-BridgeTarget returns the object -Identity names, or throws "couldn't be found".
# The directory is found in Active Directory first (a missing one is "not
# found"), then read in full by its identity: any failure there is an IIS error.
function Get-BridgeTarget {
    if ($Noun -eq "ClientAccessService") {
        return Invoke-BridgeRetry { Get-ClientAccessService -Identity $Identity }
    }
    if ($Identity -like "*\*") {
        $dir = Invoke-BridgeRetry { & "Get-$Noun" -Identity $Identity -ADPropertiesOnly }
    } else {
        $dir = @(Invoke-BridgeRetry { & "Get-$Noun" -Server $Identity -ADPropertiesOnly }) |
            Where-Object { [string]$_.Name -like "* (Default Web Site)" } | Select-Object -First 1
    }
    if ($null -eq $dir) { throw "$Noun of the Default Web Site of '$Identity' couldn't be found." }
    try {
        Invoke-BridgeRetry { & "Get-$Noun" -Identity ([string]$dir.Identity) }
    } catch {
        $script:IisReadFailed = $true
        throw "Could not read the $Noun of '$Identity' from IIS: $($_.Exception.Message)"
    }
}

function Read-BridgeTarget {
    $object = Get-BridgeTarget
    $out = [ordered]@{ Name = $Identity }
    foreach ($p in $props) { $out[$p] = ConvertTo-BridgeValue (Get-BridgeProp $object $p) }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try { Read-BridgeTarget } catch {
                if (-not $IisReadFailed -and (Test-BridgeNotFound $_.Exception.Message)) { exit 0 }
                throw
            }
        }
        { $_ -in "Create", "Update" } {
            if ($splat.Count -gt 0) {
                $target = [string](Get-BridgeTarget).Identity
                Invoke-BridgeRetry { & "Set-$Noun" -Identity $target @splat | Out-Null }
            }
            Read-BridgeTarget
        }
        "Delete" { "{}" }
    }
} catch {
    [Console]::Error.WriteLine("$Noun $Action failed: $($_.Exception.Message)")
    exit 1
}
