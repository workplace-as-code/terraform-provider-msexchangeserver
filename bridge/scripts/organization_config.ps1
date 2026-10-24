# organization_config.ps1 — Read/write Exchange organization configuration
# Singleton resource - there is exactly one organization config per organization.
# Cmdlet noun: OrganizationConfig → Get-OrganizationConfig / Set-OrganizationConfig
# Called by the Terraform provider via the Go client bridge.
# Always outputs valid JSON to stdout.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Set")] [string]$Action,

    # Writable settings
    [string]$ReadTrackingEnabled                = "",
    [string]$MailTipsAllTipsEnabled             = "",
    [string]$MailTipsExternalRecipientsTipsEnabled = "",
    [string]$MailTipsLargeAudienceThreshold     = "",
    [string]$OAuth2ClientProfileEnabled         = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password

function Get-PropSafe {
    param($Obj, [string]$Prop, $Default = $null)
    $p = $Obj.PSObject.Properties[$Prop]
    if ($null -ne $p) { return $p.Value }
    return $Default
}

function Read-Config {
    $config = Get-OrganizationConfig -ErrorAction Stop

    @{
        Identity                              = [string]$config.Identity
        DisplayName                           = if ($config.DisplayName) { $config.DisplayName } else { "" }
        SCLJunkThreshold                      = if ($null -ne (Get-PropSafe $config 'SCLJunkThreshold')) { [int](Get-PropSafe $config 'SCLJunkThreshold') } else { -1 }
        ReadTrackingEnabled                   = [bool](Get-PropSafe $config 'ReadTrackingEnabled' $false)
        MailTipsAllTipsEnabled                = [bool](Get-PropSafe $config 'MailTipsAllTipsEnabled' $true)
        MailTipsExternalRecipientsTipsEnabled = [bool](Get-PropSafe $config 'MailTipsExternalRecipientsTipsEnabled' $false)
        MailTipsLargeAudienceThreshold        = [int](Get-PropSafe $config 'MailTipsLargeAudienceThreshold' 25)
        OAuth2ClientProfileEnabled            = [bool](Get-PropSafe $config 'OAuth2ClientProfileEnabled' $true)
    } | ConvertTo-Json -Compress
}

switch ($Action) {

    "Get" {
        Read-Config
    }

    "Set" {
        $params = @{}

        if ($ReadTrackingEnabled -ne "")    { $params["ReadTrackingEnabled"]    = ($ReadTrackingEnabled -eq "true") }
        if ($MailTipsAllTipsEnabled -ne "") { $params["MailTipsAllTipsEnabled"] = ($MailTipsAllTipsEnabled -eq "true") }
        if ($MailTipsExternalRecipientsTipsEnabled -ne "") {
            $params["MailTipsExternalRecipientsTipsEnabled"] = ($MailTipsExternalRecipientsTipsEnabled -eq "true")
        }
        if ($MailTipsLargeAudienceThreshold -ne "") {
            $params["MailTipsLargeAudienceThreshold"] = [int]$MailTipsLargeAudienceThreshold
        }
        if ($OAuth2ClientProfileEnabled -ne "") { $params["OAuth2ClientProfileEnabled"] = ($OAuth2ClientProfileEnabled -eq "true") }

        if ($params.Count -gt 0) {
            Set-OrganizationConfig @params -ErrorAction Stop
        }

        Read-Config
    }
}
