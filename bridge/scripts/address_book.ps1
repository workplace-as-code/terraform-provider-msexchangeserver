# address_book.ps1 - CRUD for address lists, global address lists, offline
# address books and address book policies, with the same parameters as
# object.ps1.
# Address book specifics:
#   - Exchange returns references to address lists, GALs and offline address
#     books as identities ("\Sales", "\Sites\Paris"): they go back to Terraform
#     as names, as the configuration writes them. An address list also gets its
#     Container, the name of its parent list, from its identity.
#   - the generating mailbox comes back as a canonical name
#     ("contoso.com/Users/SystemMailbox{...}"): it goes back as the mailbox name
#   - Exchange refuses "" for a reference or a container: "" is sent as $null,
#     which clears it
#   - web distribution is refused while virtual directories are set: turning it
#     on clears them first
#   - Remove-OfflineAddressBook on the default offline address book crashes
#     Exchange (Watson dump): the script refuses it first
#   - Update-AddressList, Update-GlobalAddressList and Update-OfflineAddressBook
#     are never run: recalculating members is an administrator's task

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [Parameter(Mandatory)] [ValidateSet("AddressList","GlobalAddressList","OfflineAddressBook","AddressBookPolicy")]
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

foreach ($k in @($splat.Keys)) { if ($splat[$k] -is [string] -and $splat[$k] -eq "") { $splat[$k] = $null } }

# Get-BridgeLeafName returns the last part of an identity: "\Sites\Paris" gives "Paris".
function Get-BridgeLeafName([string]$Value) {
    $parts = @($Value -split '[\\/]' | Where-Object { $_ })
    if ($parts.Count -eq 0) { return "" }
    $parts[-1]
}

# Get-BridgeParentName returns the parent of an identity: "\Sites\Paris" gives
# "Sites", "\Top" gives "".
function Get-BridgeParentName([string]$Value) {
    $parts = @($Value -split '\\' | Where-Object { $_ })
    if ($parts.Count -lt 2) { return "" }
    $parts[-2]
}

function Read-BridgeAddressBook {
    $object = Invoke-BridgeRetry { @(& "Get-$Noun" -Identity $Identity) | Select-Object -First 1 }
    if ($null -eq $object) { throw "$Noun '$Identity' couldn't be found." }
    $out = [ordered]@{ Name = [string](Get-BridgeProp $object "Name" "") }
    foreach ($p in $props) {
        $value = Get-BridgeProp $object $p
        if ($p -eq "Container") {
            $out[$p] = Get-BridgeParentName ([string](Get-BridgeProp $object "Identity" ""))
        } elseif ($p -eq "AddressLists") {
            $out[$p] = @(foreach ($v in @($value)) { if ($null -ne $v -and "$v" -ne "") { Get-BridgeLeafName "$v" } })
        } elseif ($p -in "GlobalAddressList", "OfflineAddressBook", "RoomList", "GeneratingMailbox") {
            $out[$p] = if ($null -eq $value) { "" } else { Get-BridgeLeafName "$value" }
        } else {
            $out[$p] = ConvertTo-BridgeValue $value
        }
    }
    ConvertTo-Json -Compress -Depth 4 -InputObject $out
}

try {
    switch ($Action) {
        "Get" {
            try {
                Read-BridgeAddressBook
            } catch {
                if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
                throw
            }
        }

        "Create" {
            $new = @{}
            foreach ($k in $splat.Keys) { if ($null -ne $splat[$k]) { $new[$k] = $splat[$k] } }
            Invoke-BridgeRetry -Create { & "New-$Noun" -Name $Identity @new | Out-Null }
            Read-BridgeAddressBook
        }

        "Update" {
            $splat.Remove("Container")
            if ($Noun -eq "OfflineAddressBook" -and $splat["GlobalWebDistributionEnabled"] -eq $true -and
                -not $splat.ContainsKey("VirtualDirectories")) {
                Invoke-BridgeRetry { Set-OfflineAddressBook -Identity $Identity -VirtualDirectories $null | Out-Null }
            }
            if ($splat.Count -gt 0) { Invoke-BridgeRetry { & "Set-$Noun" -Identity $Identity @splat | Out-Null } }
            Read-BridgeAddressBook
        }

        "Delete" {
            try {
                if ($Noun -eq "OfflineAddressBook") {
                    $oab = Invoke-BridgeRetry { @(Get-OfflineAddressBook -Identity $Identity) | Select-Object -First 1 }
                    if ($null -ne $oab -and (Get-BridgeProp $oab "IsDefault" $false) -eq $true) {
                        [Console]::Error.WriteLine("OfflineAddressBook Delete failed: '$Identity' is the default " +
                            "offline address book: make another one the default first.")
                        exit 1
                    }
                }
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
