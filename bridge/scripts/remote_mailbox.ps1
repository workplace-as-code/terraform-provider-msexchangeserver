# remote_mailbox.ps1 - Read-only data source for remote mailboxes (Get-RemoteMailbox):
# the on-premises objects of mailboxes hosted in Exchange Online, in a hybrid
# organization.
# Called by the Terraform provider via the Go client bridge.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get")] [string]$Action,
    [Parameter(Mandatory)] [string]$Identity
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

# ConvertTo-BridgeSmtp strips the "smtp:" or "SMTP:" prefix of a proxy address.
function ConvertTo-BridgeSmtp {
    param($Address)
    if ($null -eq $Address) { return "" }
    "$Address" -replace '^[Ss][Mm][Tt][Pp]:', ''
}

try {
    $mailbox = Get-RemoteMailbox -Identity $Identity -ErrorAction Stop
} catch {
    # Only "object not found" means the mailbox does not exist; any other error
    # (auth, network) must fail the read.
    if (Test-BridgeNotFound $_.Exception.Message) { exit 0 }
    [Console]::Error.WriteLine("ERROR in remote_mailbox Get: $($_.Exception.Message)")
    exit 1
}

$smtpAddresses = @(
    Get-BridgeProp $mailbox "EmailAddresses" @() |
    Where-Object { "$_" -like "smtp:*" } |
    ForEach-Object { ConvertTo-BridgeSmtp $_ }
)

@{
    Identity                      = [string]$mailbox.Identity
    DisplayName                   = [string](Get-BridgeProp $mailbox "DisplayName" "")
    Alias                         = [string](Get-BridgeProp $mailbox "Alias" "")
    UserPrincipalName             = [string](Get-BridgeProp $mailbox "UserPrincipalName" "")
    PrimarySmtpAddress            = [string](Get-BridgeProp $mailbox "PrimarySmtpAddress" "")
    RemoteRoutingAddress          = ConvertTo-BridgeSmtp (Get-BridgeProp $mailbox "RemoteRoutingAddress")
    RecipientTypeDetails          = [string](Get-BridgeProp $mailbox "RecipientTypeDetails" "")
    ExchangeGuid                  = [string](Get-BridgeProp $mailbox "ExchangeGuid" "")
    HiddenFromAddressListsEnabled = [bool](Get-BridgeProp $mailbox "HiddenFromAddressListsEnabled" $false)
    EmailAddressPolicyEnabled     = [bool](Get-BridgeProp $mailbox "EmailAddressPolicyEnabled" $false)
    EmailAddresses                = $smtpAddresses
} | ConvertTo-Json -Compress
