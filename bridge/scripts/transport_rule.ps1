# transport_rule.ps1 — CRUD for Exchange transport rules (mail flow rules)
# Called by the Terraform provider via the Go client bridge.
#
# Output contract (shared by every bridge script):
#   - success            → JSON object on stdout, exit 0
#   - Get on missing rule → empty stdout, exit 0 (provider drops it from state)
#   - any other failure  → message on stderr, exit 1
#
# List parameters arrive as base64-encoded JSON arrays so that values containing
# commas, quotes or spaces survive the command line intact.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Create","Update","Delete")] [string]$Action,

    [string]$Identity,
    [string]$Name,
    [string]$Priority               = "",       # empty = let Exchange decide
    [string]$Enabled,  # only sent when set in the configuration
    [string]$RuleAction,
    [string]$RejectMessage,
    [string]$FromAddressContains    = "",       # base64 JSON array
    [string]$SubjectContains        = "",       # base64 JSON array
    [string]$AttachmentIsExecutable = "false"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password

# SCL value used for the "MoveToJmf" action: SCL 6 routes the message to the
# recipient's Junk Email folder.
$JunkScl = 6

function ConvertFrom-B64JsonArray([string]$Value) {
    if (-not $Value) { return @() }
    $json = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
    # Assigned first: Windows PowerShell 5.1 sends a JSON array down the
    # pipeline as one object, so @($json | ConvertFrom-Json) would nest it.
    $items = ConvertFrom-Json -InputObject $json
    return @($items)
}

function ConvertTo-StringArray($Values) {
    if ($null -eq $Values) { return @() }
    return [string[]]@($Values | ForEach-Object { $_.ToString() })
}

$AttachmentIsExecutableBool = $AttachmentIsExecutable -eq "true"
$FromAddressArr             = @(ConvertFrom-B64JsonArray $FromAddressContains)
$SubjectArr                 = @(ConvertFrom-B64JsonArray $SubjectContains)

# Get-RuleProp reads a rule property that may not exist: Edge Transport rules
# have other conditions and actions than the rules of the organization, and
# StrictMode throws on a missing property.
function Get-RuleProp($Rule, [string]$Name) {
    $p = $Rule.PSObject.Properties[$Name]
    if ($null -ne $p) { $p.Value }
}

# Map a rule object to the JSON schema returned to Terraform.
function ConvertTo-BridgeRule {
    param($Rule)

    # Mailbox rules reject with an NDR (RejectMessageReasonText), Edge rules
    # during the SMTP session (SmtpRejectMessageRejectText).
    $reject = Get-RuleProp $Rule "RejectMessageReasonText"
    if (-not $reject) { $reject = Get-RuleProp $Rule "SmtpRejectMessageRejectText" }
    $actionName = if ($reject) { "Reject" }
                  elseif ($BridgeRole -eq "Edge" -and (Get-RuleProp $Rule "Quarantine")) { "Quarantine" }
                  elseif ((Get-RuleProp $Rule "SetSCL") -eq $JunkScl) { "MoveToJmf" }
                  else { "Unsupported" }

    @{
        Identity               = "$($Rule.Identity)"
        Name                   = $Rule.Name
        Priority               = $Rule.Priority
        Enabled                = ($Rule.State -eq "Enabled")
        Action                 = $actionName
        RejectMessage          = if ($reject) { "$reject" } else { "" }
        FromAddressContains    = @(ConvertTo-StringArray (Get-RuleProp $Rule "FromAddressContainsWords"))
        SubjectContains        = @(ConvertTo-StringArray (Get-RuleProp $Rule "SubjectContainsWords"))
        AttachmentIsExecutable = [bool](Get-RuleProp $Rule "AttachmentHasExecutableContent")
    } | ConvertTo-Json -Compress
}

# Builds the action and condition parameters. On Update, the conditions and
# the previous action that the rule still has but the plan removed are cleared
# ($null, or $false for a switch) so Exchange matches the plan.
#
# Only what the current rule actually uses is cleared. Each role rejects the
# parameters of the other, even to clear them: Mailbox servers reject
# -Quarantine and -SmtpRejectMessageRejectText ("isn't valid on a server with
# the Hub Transport role"), Edge Transport servers reject the organization
# conditions and actions such as -AttachmentHasExecutableContent and
# -RejectMessageReasonText ("isn't valid on an Edge Transport server").
# Get-Command cannot tell: the Edge snap-in declares every parameter.
function Get-RuleParams([bool]$IsUpdate, $Current = $null) {
    $p = @{}

    switch ($RuleAction) {
        "Reject"     {
            if ($BridgeRole -eq "Edge") { $p["SmtpRejectMessageRejectText"] = $RejectMessage }
            else                        { $p["RejectMessageReasonText"] = $RejectMessage }
        }
        "Quarantine" { $p["Quarantine"] = $true }
        "MoveToJmf"  { $p["SetSCL"] = $JunkScl }
        default      { throw "Unsupported action '$RuleAction'. Valid values: Reject, Quarantine, MoveToJmf." }
    }

    if ($FromAddressArr.Count -gt 0)  { $p["FromAddressContainsWords"] = [string[]]$FromAddressArr }
    if ($SubjectArr.Count -gt 0)      { $p["SubjectContainsWords"] = [string[]]$SubjectArr }
    if ($AttachmentIsExecutableBool)  { $p["AttachmentHasExecutableContent"] = $true }

    if ($IsUpdate) {
        $clear = @{
            RejectMessageReasonText        = $null
            SmtpRejectMessageRejectText    = $null
            Quarantine                     = $false
            SetSCL                         = $null
            FromAddressContainsWords       = $null
            SubjectContainsWords           = $null
            AttachmentHasExecutableContent = $false
        }
        foreach ($k in $clear.Keys) {
            if ($p.ContainsKey($k)) { continue }
            $value = Get-RuleProp $Current $k
            $inUse = if ($value -is [bool]) { $value } else { @($value | Where-Object { $null -ne $_ -and "$_" -ne "" }).Count -gt 0 }
            if ($inUse) { $p[$k] = $clear[$k] }
        }
    }

    # Safety net, duplicated from the provider's plan-time validation: a rule
    # without conditions applies to every message in the organization.
    if ($FromAddressArr.Count -eq 0 -and $SubjectArr.Count -eq 0 -and -not $AttachmentIsExecutableBool) {
        throw "Refusing to write transport rule '$Name' without any condition: it would apply to all mail."
    }
    return $p
}

try {
    switch ($Action) {

        "Get" {
            try {
                $rule = Get-TransportRule -Identity $Identity -ErrorAction Stop
            } catch {
                if ($_.Exception.Message -match "couldn't be found|not found|ObjectNotFound") { exit 0 }
                throw
            }
            ConvertTo-BridgeRule -Rule $rule
        }

        "Create" {
            $params = Get-RuleParams -IsUpdate $false
            $params["Name"] = $Name
            # Create disabled when requested, so the rule never runs unwanted.
            if ($PSBoundParameters.ContainsKey("Enabled")) { $params["Enabled"] = ($Enabled -eq "true") }
            if ($Priority -ne "") { $params["Priority"] = [int]$Priority }

            $rule = New-TransportRule @params
            ConvertTo-BridgeRule -Rule (Get-TransportRule -Identity $rule.Identity -ErrorAction Stop)
        }

        "Update" {
            $current = Get-TransportRule -Identity $Identity -ErrorAction Stop
            $params = Get-RuleParams -IsUpdate $true -Current $current
            $params["Identity"] = $Identity
            if ($Name -and $Name -ne $Identity) { $params["Name"] = $Name }
            if ($Priority -ne "") { $params["Priority"] = [int]$Priority }

            Set-TransportRule @params

            $newIdentity = if ($params.ContainsKey("Name")) { $Name } else { $Identity }
            if ($PSBoundParameters.ContainsKey("Enabled")) {
                if ($Enabled -eq "true") { Enable-TransportRule -Identity $newIdentity @BridgeNoConfirm }
                else                     { Disable-TransportRule -Identity $newIdentity @BridgeNoConfirm }
            }

            ConvertTo-BridgeRule -Rule (Get-TransportRule -Identity $newIdentity -ErrorAction Stop)
        }

        "Delete" {
            try {
                Remove-TransportRule -Identity $Identity @BridgeNoConfirm -ErrorAction Stop
            } catch {
                # Already gone is a successful delete; anything else is a real error.
                if ($_.Exception.Message -notmatch "couldn't be found|not found|ObjectNotFound") { throw }
            }
            Write-Output "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("transport_rule $Action failed: $($_.Exception.Message)")
    exit 1
}
