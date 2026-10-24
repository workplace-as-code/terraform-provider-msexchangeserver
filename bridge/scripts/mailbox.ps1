# mailbox.ps1 — Read-only data source for Exchange mailboxes
# Cmdlet noun: Mailbox → Get-Mailbox
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

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

$mailbox = Get-Mailbox -Identity $Identity -ErrorAction Stop

# Extract only SMTP proxy addresses (strip the "smtp:" prefix)
$smtpAddresses = @(
    $mailbox.EmailAddresses |
    Where-Object { $_.ToString() -like "smtp:*" -or $_.ToString() -like "SMTP:*" } |
    ForEach-Object { $_.ToString() -replace '^[Ss][Mm][Tt][Pp]:', '' }
)

@{
    Identity                      = [string]$mailbox.Identity
    DisplayName                   = $mailbox.DisplayName
    PrimarySmtpAddress            = $mailbox.PrimarySmtpAddress.ToString()
    RecipientTypeDetails          = [string]$mailbox.RecipientTypeDetails
    HiddenFromAddressListsEnabled = $mailbox.HiddenFromAddressListsEnabled
    LitigationHoldEnabled         = $mailbox.LitigationHoldEnabled
    RetentionPolicy               = if ($mailbox.RetentionPolicy) { [string]$mailbox.RetentionPolicy } else { "" }
    EmailAddresses                = $smtpAddresses
} | ConvertTo-Json -Compress
