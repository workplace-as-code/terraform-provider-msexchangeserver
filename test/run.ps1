<#
.SYNOPSIS
    Integration test of the provider against an Exchange Server organization
    (mode = "onprem").

.DESCRIPTION
    Runs from a Windows machine joined to the Active Directory domain, with
    terraform.exe on the PATH and a build of the provider
    (terraform-provider-msexchangeserver.exe).

    -Step check (read-only):
        Kerberos connection check, then terraform plan: the provider connects,
        reads the data sources and the organization-wide settings, and changes
        nothing.

    -Step all (default): the full cycle of test/run.sh:
        pre-cleanup -> apply -> drift check -> update -> drift check ->
        import of every resource -> drift check -> terraform query ->
        destroy -> deletion check

    Test objects are named TF-CI-*. The organization config and the transport
    config are only imported, never changed. Everything is written to a log
    file next to this script: send it back when a step fails.

.PARAMETER Server
    FQDN of an Exchange server, as registered in Active Directory.

.PARAMETER Username
    Account with the Organization Management role group, as DOMAIN\user or
    user@domain. The password is asked for. Use -Username or -Integrated.

.PARAMETER Integrated
    Integrated Windows authentication (auth_method = "integrated"): the
    provider connects as the account running this script, with no password.
    Run it from a session of that account (a service or scheduled task, or
    runas). Use -Username or -Integrated.

.PARAMETER ProviderPath
    Path of terraform-provider-msexchangeserver.exe. Defaults to the file next
    to this script.

.PARAMETER OneShot
    Starts one PowerShell process per operation instead of keeping a few open
    (EXCHANGE_BRIDGE_MODE=oneshot). For troubleshooting.

.PARAMETER DebugLog
    Also writes Terraform's debug log (TF_LOG=DEBUG) next to the log file.

.EXAMPLE
    .\run.ps1 -Server exch01.contoso.local -Username CONTOSO\administrator -Step check

.EXAMPLE
    .\run.ps1 -Server exch01.contoso.local -Username CONTOSO\administrator

.EXAMPLE
    .\run.ps1 -Server exch01.contoso.local -Integrated
#>
param(
    [Parameter(Mandatory = $true)] [string]$Server,
    [string]$Username,
    [switch]$Integrated,
    [ValidateSet("check", "all")] [string]$Step = "all",
    [string]$ProviderPath = (Join-Path $PSScriptRoot "terraform-provider-msexchangeserver.exe"),
    [switch]$OneShot,
    [switch]$DebugLog
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$Log = Join-Path $PSScriptRoot "run-$stamp.log"

# Objects created by the test, as resource address = import ID.
$Created = [ordered]@{
    "msexchangeserver_accepted_domain.test"         = "TF-CI-AcceptedDomain"
    "msexchangeserver_remote_domain.test"           = "TF-CI-Domain"
    "msexchangeserver_transport_rule.test"          = "TF-CI-Rule"
    "msexchangeserver_owa_mailbox_policy.test"      = "TF-CI-OWA"
    "msexchangeserver_mobile_device_mailbox_policy.test" = "TF-CI-Mobile"
    "msexchangeserver_email_address_policy.test"    = "TF-CI-AddressPolicy"
    "msexchangeserver_retention_policy.test"        = "TF-CI-Retention"
    "msexchangeserver_retention_policy_tag.test"    = "TF-CI-RetentionTag"
}
# Organization-wide objects imported by main.tf and never destroyed.
$Keep = @("msexchangeserver_organization_config.this", "msexchangeserver_transport_config.this")
# Resource types listed by terraform query.
$ListTypes = @("accepted_domain", "remote_domain", "transport_rule", "owa_mailbox_policy",
    "mobile_device_mailbox_policy", "email_address_policy", "retention_policy",
    "retention_policy_tag", "organization_config", "transport_config")

function Write-Log([string[]]$Lines) { if ($Lines.Count -gt 0) { Add-Content -Path $Log -Value $Lines -Encoding UTF8 } }
function Write-Step([string]$Message) { Write-Host ""; Write-Host "==> $Message" -ForegroundColor Yellow; Write-Log "", "==> $Message" }
function Write-Pass([string]$Message) { Write-Host "  OK   $Message" -ForegroundColor Green; Write-Log "  OK   $Message" }
function Write-Info([string]$Message) { Write-Host "       $Message" -ForegroundColor Cyan; Write-Log "       $Message" }
function Stop-Test([string]$Message) {
    Write-Host "  FAIL $Message" -ForegroundColor Red
    Write-Log "  FAIL $Message"
    throw $Message
}

# Invoke-Tf runs terraform with the given arguments and returns its exit code
# and output. Windows PowerShell turns native stderr into error records, which
# would stop the script: errors are handled through the exit code instead.
function Invoke-Tf {
    $tfArgs = @($args)
    Write-Log "> terraform $($tfArgs -join ' ')"
    $previous = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        # Windows PowerShell wraps each stderr line in an ErrorRecord whose text
        # is in TargetObject ("System.Management.Automation.RemoteException"
        # otherwise, for empty lines).
        $output = @(& terraform @tfArgs 2>&1 | ForEach-Object {
            if ($_ -is [System.Management.Automation.ErrorRecord]) {
                if ($null -ne $_.TargetObject) { "$($_.TargetObject)" } else { $_.Exception.Message }
            } else { "$_" }
        })
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previous
    }
    Write-Log $output
    return New-Object PSObject -Property @{ Code = $code; Output = $output }
}

function Assert-Tf($Result, [string]$Label) {
    if ($Result.Code -ne 0) {
        $Result.Output | Select-Object -Last 30 | ForEach-Object { Write-Host "       $_" }
        Stop-Test "$Label failed (exit $($Result.Code)), see $Log"
    }
}

# Assert-NoChanges fails unless terraform plan reports no changes.
function Assert-NoChanges([string]$Label, [string[]]$Extra = @()) {
    $r = Invoke-Tf plan -detailed-exitcode -no-color @Extra
    switch ($r.Code) {
        0 { Write-Pass "$Label`: no changes" }
        2 {
            $r.Output | Where-Object { $_ -match '^\s+[~+-]|will be|must be' } | ForEach-Object { Write-Host "       $_" }
            Stop-Test "$Label`: the plan shows changes, see $Log"
        }
        default { Assert-Tf $r "$Label (terraform plan)" }
    }
}

# Import-Orphans imports every test object that still exists and returns the
# matching -target arguments. Only "Cannot import non-existent remote object"
# means the object is gone (the error then only goes to the log): any other
# failure, such as a connection or read error, stops the test, or an object
# left behind would go unnoticed.
function Import-Orphans {
    $targets = @()
    foreach ($address in $Created.Keys) {
        $r = Invoke-Tf import -no-color $address $Created[$address]
        if ($r.Code -eq 0) { $targets += "-target=$address"; continue }
        if (-not ($r.Output -match "Cannot import non-existent remote object")) {
            Assert-Tf $r "Import of $address to check whether it exists"
        }
    }
    return $targets
}

# Remove-TestDir deletes a temporary folder. The provider's PowerShell
# processes can still use it as their current folder for a few seconds after
# Terraform exits (longer over WinRM, while they close their remote session):
# retry, and only warn if it stays locked, so cleanup never stops the test.
function Remove-TestDir([string]$Path) {
    for ($i = 0; $i -lt 15; $i++) {
        try {
            Remove-Item -Recurse -Force -LiteralPath $Path -ErrorAction Stop
            return
        } catch {
            Start-Sleep -Seconds 2
        }
    }
    Write-Info "Could not delete $Path (still in use): delete it later"
}

function Reset-State {
    Remove-Item -Path (Join-Path $PSScriptRoot "terraform.tfstate*") -Force -ErrorAction SilentlyContinue
}

$pushed = $false
try {
    $account = "$([Environment]::UserDomainName)\$([Environment]::UserName)"
    $authText = if ($Integrated) { "-Integrated (as $account)" } else { "-Username $Username" }
    Write-Log "run.ps1 -Server $Server $authText -Step $Step, $(Get-Date -Format o)"
    Write-Log "PowerShell $($PSVersionTable.PSVersion) $($PSVersionTable.PSEdition), $([Environment]::OSVersion.VersionString)"

    # -----------------------------------------------------------------------
    Write-Step "Checking prerequisites"
    if (-not (Get-Command terraform -ErrorAction SilentlyContinue)) { Stop-Test "terraform.exe is not on the PATH" }
    $tfVersion = (Invoke-Tf version -json).Output -join "" | ConvertFrom-Json
    Write-Pass "Terraform $($tfVersion.terraform_version)"
    if (-not (Test-Path -LiteralPath $ProviderPath -PathType Leaf)) { Stop-Test "Provider not found: $ProviderPath" }
    if ((Split-Path -Leaf $ProviderPath) -ne "terraform-provider-msexchangeserver.exe") {
        Stop-Test "The provider file must be named terraform-provider-msexchangeserver.exe"
    }
    Write-Pass "Provider: $ProviderPath"
    if (-not (Get-Command powershell.exe -ErrorAction SilentlyContinue)) { Stop-Test "powershell.exe (Windows PowerShell 5.1) not found" }
    if ([bool]$Username -eq [bool]$Integrated) { Stop-Test "Use either -Username (password asked for) or -Integrated (the account running this script)" }
    if ($Server -notmatch '\.' -or $Server -match '^\d+(\.\d+){3}$') {
        Stop-Test "-Server must be the FQDN of the Exchange server (Kerberos does not work with an IP address or a short name)"
    }

    # -----------------------------------------------------------------------
    Write-Step "Kerberos connection to http://$Server/PowerShell/"
    # Integrated: no credential, New-PSSession uses the Kerberos ticket of this session
    $credential = @{}
    if ($Integrated) {
        Write-Info "Integrated authentication as $account"
    } else {
        $cred = Get-Credential -UserName $Username -Message "Exchange Server account (Organization Management)"
        if ($null -eq $cred) { Stop-Test "No credential given" }
        $credential = @{ Credential = $cred }
    }
    $session = New-PSSession -ConfigurationName Microsoft.Exchange -ConnectionUri "http://$Server/PowerShell/" `
        -Authentication Kerberos @credential
    try {
        # The Exchange endpoint runs remote commands without the PowerShell
        # language (NoLanguage mode): only plain cmdlet calls, results processed here.
        $domains = @(Invoke-Command -Session $session -ScriptBlock { Get-AcceptedDomain })
        $servers = @(Invoke-Command -Session $session -ScriptBlock { Get-ExchangeServer })
    } finally {
        Remove-PSSession $session
    }
    $default = @($domains | Where-Object { "$($_.Default)" -eq "True" }) | Select-Object -First 1
    if ($null -eq $default) { Stop-Test "No default accepted domain found" }
    Write-Pass "Connected. Accepted domains: $(($domains | ForEach-Object { "$($_.DomainName)" }) -join ', ')"
    $exchange = @($servers | Where-Object { "$($_.Fqdn)" -eq $Server }) | Select-Object -First 1
    if ($null -eq $exchange) { $exchange = $servers | Select-Object -First 1 }
    if ($null -ne $exchange) { Write-Info "Exchange server: $($exchange.Name), version $($exchange.AdminDisplayVersion)" }

    # -----------------------------------------------------------------------
    Write-Step "Configuring Terraform for this test"
    $providerDir = (Split-Path -Parent (Resolve-Path -LiteralPath $ProviderPath).Path) -replace '\\', '/'
    $tfrc = Join-Path $env:TEMP "msexchangeserver-onprem-test.tfrc"
    Set-Content -Path $tfrc -Encoding ASCII -Value @(
        'provider_installation {',
        '  dev_overrides {',
        "    `"workplace-as-code/msexchangeserver`" = `"$providerDir`"",
        '  }',
        '  direct {}',
        '}'
    )
    $env:TF_CLI_CONFIG_FILE    = $tfrc
    $env:TF_IN_AUTOMATION      = "1"
    $env:CHECKPOINT_DISABLE    = "1"
    $env:EXCHANGE_SERVER       = $Server
    if ($Integrated) {
        $env:EXCHANGE_AUTH_METHOD = "integrated"
        Remove-Item Env:EXCHANGE_USERNAME, Env:EXCHANGE_PASSWORD -ErrorAction SilentlyContinue
    } else {
        $env:EXCHANGE_AUTH_METHOD = "password"
        $env:EXCHANGE_USERNAME    = $Username
        $env:EXCHANGE_PASSWORD    = $cred.GetNetworkCredential().Password
    }
    $env:TF_VAR_default_domain = "$($default.Name)"
    if ($OneShot) { $env:EXCHANGE_BRIDGE_MODE = "oneshot" } else { Remove-Item Env:EXCHANGE_BRIDGE_MODE -ErrorAction SilentlyContinue }
    if ($DebugLog) { $env:TF_LOG = "DEBUG"; $env:TF_LOG_PATH = Join-Path $PSScriptRoot "terraform-debug-$stamp.log" }
    Write-Pass "Development override for the provider: $tfrc"

    Push-Location $PSScriptRoot
    $pushed = $true
    Remove-Item -Recurse -Force .terraform, .terraform.lock.hcl -ErrorAction SilentlyContinue
    Reset-State
    Assert-Tf (Invoke-Tf init -no-color) "terraform init"
    Write-Pass "terraform init"

    # -----------------------------------------------------------------------
    Write-Step "Read-only plan: data sources and organization-wide settings"
    $r = Invoke-Tf plan -no-color
    Assert-Tf $r "terraform plan"
    $planLine = @($r.Output | Where-Object { $_ -match '^Plan:' }) | Select-Object -First 1
    Write-Pass "terraform plan: $planLine"

    if ($Step -eq "check") {
        Write-Host ""
        Write-Host "Check passed: the provider connects to $Server and reads the organization." -ForegroundColor Green
        Write-Host "Run the full cycle with: .\run.ps1 -Server $Server $(if ($Integrated) { '-Integrated' } else { "-Username $Username" })"
        return
    }

    # -----------------------------------------------------------------------
    Write-Step "Pre-cleanup: removing test objects left by an interrupted run"
    $orphans = @(Import-Orphans)
    if ($orphans.Count -gt 0) {
        Write-Info "$($orphans.Count) object(s) found, destroying them"
        Assert-Tf (Invoke-Tf destroy -auto-approve -no-color @orphans) "Pre-cleanup destroy"
    }
    Reset-State
    Write-Pass "Clean"

    # -----------------------------------------------------------------------
    Write-Step "terraform apply"
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color) "terraform apply"
    Write-Pass "Apply"
    Assert-NoChanges "Drift check after apply"

    Write-Step "Update of every test object (phase=update)"
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color -var phase=update) "terraform apply (update)"
    Write-Pass "Update"
    Assert-NoChanges "Drift check after update" @("-var", "phase=update")

    # -----------------------------------------------------------------------
    Write-Step "Import of every test object"
    foreach ($address in $Created.Keys) {
        Assert-Tf (Invoke-Tf state rm -no-color $address) "terraform state rm $address"
        Assert-Tf (Invoke-Tf import -no-color -var phase=update $address $Created[$address]) "Import of $address"
    }
    Write-Pass "$($Created.Count) resources imported"
    # apply_to_recipients is a provider setting that Exchange does not store: an
    # imported policy has none, so the first apply only records it. It applies
    # nothing, since no setting of the policy changes.
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color -var phase=update "-target=msexchangeserver_email_address_policy.test") "Recording apply_to_recipients after import"
    Assert-NoChanges "Drift check after import" @("-var", "phase=update")

    # -----------------------------------------------------------------------
    Write-Step "terraform query: generated configuration of the organization"
    $minor = [int]($tfVersion.terraform_version -split '\.')[1]
    if ($minor -lt 14) {
        Write-Info "Skipped: terraform query needs Terraform 1.14 or later"
    } else {
        $queryDir = Join-Path $env:TEMP "msexchangeserver-onprem-query-$stamp"
        New-Item -ItemType Directory -Path $queryDir | Out-Null
        Set-Content -Path (Join-Path $queryDir "main.tf") -Encoding ASCII -Value @(
            'terraform {',
            '  required_providers {',
            '    msexchangeserver = { source = "workplace-as-code/msexchangeserver" }',
            '  }',
            '}',
            'provider "msexchangeserver" {}'
        )
        $blocks = foreach ($t in $ListTypes) { "list `"msexchangeserver_$t`" `"all`" {"; "  provider = msexchangeserver"; "}"; "" }
        Set-Content -Path (Join-Path $queryDir "all.tfquery.hcl") -Encoding ASCII -Value $blocks
        Push-Location $queryDir
        try {
            Assert-Tf (Invoke-Tf query -no-color "-generate-config-out=generated.tf") "terraform query"
            $listed = @(Get-Content generated.tf | Where-Object { $_ -match '^import \{' }).Count
            $r = Invoke-Tf plan -no-color
            $planLine = @($r.Output | Where-Object { $_ -match '^Plan:' }) | Select-Object -First 1
        } finally {
            Pop-Location
        }
        if ($planLine -ne "Plan: $listed to import, 0 to add, 0 to change, 0 to destroy.") {
            Write-Log (Get-Content (Join-Path $queryDir "generated.tf"))
            Write-Log $r.Output
            Stop-Test "The generated configuration does not match the organization: $planLine (see $queryDir)"
        }
        Remove-TestDir $queryDir
        Write-Pass "$listed objects listed; their generated configuration only plans imports"
    }

    # -----------------------------------------------------------------------
    Write-Step "terraform destroy"
    foreach ($address in $Keep) { Assert-Tf (Invoke-Tf state rm -no-color $address) "terraform state rm $address" }
    $targets = @($Created.Keys | ForEach-Object { "-target=$_" })
    Assert-Tf (Invoke-Tf destroy -auto-approve -no-color -var phase=update @targets) "terraform destroy"
    Write-Pass "Destroy"

    Write-Step "Checking that the test objects no longer exist"
    Reset-State
    $left = @(Import-Orphans)
    if ($left.Count -gt 0) {
        Invoke-Tf destroy -auto-approve -no-color @left | Out-Null
        Stop-Test "Still present after destroy (now removed): $($left -join ', ')"
    }
    Reset-State
    Write-Pass "All test objects were deleted"

    Write-Host ""
    Write-Host "All Exchange Server integration tests passed." -ForegroundColor Green
    Write-Host "  Created, updated, imported and destroyed: $($Created.Count) resources"
    Write-Host "  Imported read-only: organization_config, transport_config"
    Write-Host "  Data sources: accepted_domain, accepted_domains, organization_config"
} finally {
    if ($pushed) { Pop-Location }
    # The other test scripts of this window (edge, transport) do not set the method
    Remove-Item Env:EXCHANGE_PASSWORD, Env:EXCHANGE_AUTH_METHOD -ErrorAction SilentlyContinue
    Write-Host ""
    Write-Host "Log: $Log"
}
