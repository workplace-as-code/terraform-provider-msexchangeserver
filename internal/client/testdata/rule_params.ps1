# rule_params.ps1 - checks which parameters transport_rule.ps1 sends on update
# (run by scripts_test.go). Each role rejects the parameters of the other, even
# to clear them: only what the current rule uses may be cleared.
param([string]$Script)
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Script, [ref]$null, [ref]$null)
foreach ($f in $ast.FindAll({ $args[0] -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true)) {
    if ($f.Name -in "Get-RuleProp", "Get-RuleParams") { . ([scriptblock]::Create($f.Extent.Text)) }
}
$JunkScl = 6
function Check($Label, $Params, [string[]]$Want) {
    $got = @($Params.Keys | Sort-Object) -join ","
    $exp = @($Want | Sort-Object) -join ","
    if ($got -eq $exp) { "PASS $Label" } else { "FAIL $Label`: got [$got] want [$exp]" }
}

# Edge: Quarantine rule with a subject -> Reject (SMTP rejection) with two subjects.
$BridgeRole = "Edge"
$edgeRule = [pscustomobject]@{ Quarantine = $true; SetSCL = $null; RejectMessageReasonText = $null; SubjectContainsWords = @("[TF-CI-TEST]"); FromAddressContainsWords = @() }
$RuleAction = "Reject"; $RejectMessage = "x"; $SubjectArr = @("a", "b"); $FromAddressArr = @(); $AttachmentIsExecutableBool = $false
$p = Get-RuleParams -IsUpdate $true -Current $edgeRule
Check "Edge Quarantine -> Reject" $p @("SmtpRejectMessageRejectText", "SubjectContainsWords", "Quarantine")
if ($p["Quarantine"] -ne $false) { "FAIL Quarantine must be cleared with false" }

# Edge: Reject -> Quarantine clears the SMTP rejection text.
$edgeReject = [pscustomobject]@{ Quarantine = $false; SetSCL = $null; SmtpRejectMessageRejectText = "no"; SubjectContainsWords = @("a") }
$RuleAction = "Quarantine"; $SubjectArr = @("a")
$p = Get-RuleParams -IsUpdate $true -Current $edgeReject
Check "Edge Reject -> Quarantine" $p @("Quarantine", "SubjectContainsWords", "SmtpRejectMessageRejectText")

$BridgeRole = "Mailbox"
# Mailbox: Reject rule with executable attachments -> subject only (no Quarantine property sent).
$mbxRule = [pscustomobject]@{ Quarantine = $false; SetSCL = $null; RejectMessageReasonText = "no"; SubjectContainsWords = $null; FromAddressContainsWords = $null; AttachmentHasExecutableContent = $true }
$RuleAction = "Reject"; $SubjectArr = @("a")
$p = Get-RuleParams -IsUpdate $true -Current $mbxRule
Check "Mailbox attachment removed" $p @("RejectMessageReasonText", "SubjectContainsWords", "AttachmentHasExecutableContent")

# Mailbox: MoveToJmf (SCL 6) -> Reject clears SetSCL.
$jmfRule = [pscustomobject]@{ Quarantine = $false; SetSCL = 6; RejectMessageReasonText = $null; SubjectContainsWords = @("a"); FromAddressContainsWords = $null; AttachmentHasExecutableContent = $false }
$p = Get-RuleParams -IsUpdate $true -Current $jmfRule
Check "MoveToJmf -> Reject" $p @("RejectMessageReasonText", "SubjectContainsWords", "SetSCL")

# Create sends no clearing parameter.
$p = Get-RuleParams -IsUpdate $false
Check "Create" $p @("RejectMessageReasonText", "SubjectContainsWords")
