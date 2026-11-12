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
    [ValidateSet("Identity", "Name", "IPRange")] [string]$Property = "Identity"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

try {
    $cmdlet = Get-Command "Get-$Noun"
    # Some Get cmdlets return the first 1000 objects only by default.
    $size = if ($cmdlet.Parameters.ContainsKey("ResultSize")) { @{ ResultSize = "Unlimited" } } else { @{} }
    # Virtual directories: the identity is in Active Directory, reading IIS on
    # every server is slow and fails when one does not answer.
    if ($cmdlet.Parameters.ContainsKey("ADPropertiesOnly")) { $size["ADPropertiesOnly"] = $true }
    # Some Get cmdlets report "couldn't be found" when there is no object.
    $objects = try { @(Invoke-BridgeRetry { & "Get-$Noun" @size }) } catch {
        if (-not (Test-BridgeNotFound $_.Exception.Message)) { throw }
        @()
    }
    # Exchange certificates come back from the remote session as plain
    # X509Certificate2 objects, without their Identity: select it on the
    # server (see exchange_certificate.ps1).
    $session = @(Get-PSSession | Where-Object { $_.ConfigurationName -eq "Microsoft.Exchange" -and $_.State -eq "Opened" }) |
        Select-Object -First 1
    if ($Noun -eq "ExchangeCertificate" -and $null -ne $session) {
        $objects = @(Invoke-Command -Session $session -ScriptBlock { Get-ExchangeCertificate | Select-Object Identity } -ErrorAction Stop)
    }
    # On a Mailbox connection, Get-TransportService also returns the Edge
    # servers, from the copy EdgeSync keeps in Active Directory:
    # server_settings.ps1 refuses them there.
    if ($Noun -eq "TransportService" -and $BridgeRole -ne "Edge") {
        $edgeServers = @(Invoke-BridgeRetry { Get-ExchangeServer } | Where-Object { [bool](Get-BridgeProp $_ "IsEdgeServer" $false) } |
            ForEach-Object { [string]$_.Name })
        $objects = @($objects | Where-Object { $edgeServers -notcontains [string](Get-BridgeProp $_ "Name" "") })
    }
    $ids = @(foreach ($object in $objects) { [string](Get-BridgeProp $object $Property "") })
    ConvertTo-Json -Compress -InputObject ([string[]]@($ids | Where-Object { $_ }))
} catch {
    [Console]::Error.WriteLine("Listing $Noun failed: $($_.Exception.Message)")
    exit 1
}
