<#
.SYNOPSIS
    Integration test of the provider against an Edge Transport server
    (role = "Edge").

.DESCRIPTION
    Two ways to reach the Edge server:
      - locally, without -Server: run this script on the Edge server itself,
        in an elevated PowerShell, with terraform.exe on the PATH;
      - remotely, with -Server and -Username: run it from another Windows
        machine, which opens a WinRM session (Negotiate) with a local
        administrator of the Edge server.

    -Step check (read-only): connection check, a plan that must refuse an
    organization resource on Edge, then a plan of main.tf.

    -Step all (default): pre-cleanup -> apply -> drift check -> update ->
    drift check -> import -> drift check -> terraform query of the Edge
    resource types -> destroy -> deletion check.

    The only object created is a disabled transport rule, TF-CI-EdgeRule.
    Accepted and remote domains (copied by EdgeSync) and the transport config
    are only read. Everything is written to a log file next to this script.

.PARAMETER Server
    Name of the Edge server for a remote test. Leave it out to test locally.

.PARAMETER Username
    Local administrator of the Edge server, as EDGE01\Administrator. Needed
    with -Server; the password is asked for.

.PARAMETER ProviderPath
    Path of terraform-provider-msexchangeserver.exe. Defaults to the file in
    the parent folder (the root of the test kit).

.EXAMPLE
    .\run.ps1 -Step check

.EXAMPLE
    .\run.ps1 -Server edge01.perimeter.contoso.com -Username EDGE01\Administrator
#>
param(
    [string]$Server = "",
    [string]$Username = "",
    [ValidateSet("check", "all")] [string]$Step = "all",
    [string]$ProviderPath = (Join-Path (Split-Path -Parent $PSScriptRoot) "terraform-provider-msexchangeserver.exe"),
    [switch]$OneShot,
    [switch]$DebugLog
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$Log = Join-Path $PSScriptRoot "run-edge-$stamp.log"
$Mode = if ($Server) { "remote" } else { "local" }

# Objects created by the test, as resource address = import ID.
$Created = [ordered]@{ "msexchangeserver_transport_rule.test" = "TF-CI-EdgeRule" }
# Imported by main.tf and never destroyed.
$Keep = @("msexchangeserver_transport_config.this")
# Resource types available on Edge, listed by terraform query.
$ListTypes = @("accepted_domain", "remote_domain", "transport_rule", "transport_config")

function Write-Log([string[]]$Lines) { if ($Lines.Count -gt 0) { Add-Content -Path $Log -Value $Lines -Encoding UTF8 } }
function Write-Step([string]$Message) { Write-Host ""; Write-Host "==> $Message" -ForegroundColor Yellow; Write-Log "", "==> $Message" }
function Write-Pass([string]$Message) { Write-Host "  OK   $Message" -ForegroundColor Green; Write-Log "  OK   $Message" }
function Write-Info([string]$Message) { Write-Host "       $Message" -ForegroundColor Cyan; Write-Log "       $Message" }
function Stop-Test([string]$Message) {
    Write-Host "  FAIL $Message" -ForegroundColor Red
    Write-Log "  FAIL $Message"
    throw $Message
}

# Invoke-Tf runs terraform and returns its exit code and output (see
# test/run.ps1: native stderr must not stop the script).
function Invoke-Tf {
    $tfArgs = @($args)
    Write-Log "> terraform $($tfArgs -join ' ')"
    $previous = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
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

# Import-Orphans imports the test objects that still exist and returns the
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

# The Exchange snap-in name changed across versions: take the registered one.
$SnapinFilter = { $_.Name -like "Microsoft.Exchange.Management.PowerShell.*" }

$pushed = $false
try {
    Write-Log "run.ps1 (Edge, $Mode) -Server '$Server' -Username '$Username' -Step $Step, $(Get-Date -Format o)"
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
    if ($Server -and -not $Username) { Stop-Test "-Username is required with -Server (a local administrator of the Edge server)" }

    # -----------------------------------------------------------------------
    if ($Mode -eq "local") {
        Write-Step "Local connection: Exchange snap-in on this Edge server"
        $admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
            [Security.Principal.WindowsBuiltInRole]::Administrator)
        if (-not $admin) { Stop-Test "Run this script in an elevated PowerShell (Run as administrator)" }
        $snapin = Get-PSSnapin -Registered | Where-Object $SnapinFilter | Select-Object -First 1
        if ($null -eq $snapin) { Stop-Test "The Exchange management tools are not installed on this machine: run the local test on the Edge server" }
        if (-not (Get-PSSnapin -Name $snapin.Name -ErrorAction SilentlyContinue)) { Add-PSSnapin $snapin.Name }
        $domains = @(Get-AcceptedDomain | ForEach-Object { "$($_.DomainName)" })
        $subscriptions = @(Get-EdgeSubscription).Count
        $edgeName = $env:COMPUTERNAME
    } else {
        Write-Step "WinRM connection to $Server (Negotiate, local account)"
        $cred = Get-Credential -UserName $Username -Message "Local administrator of the Edge server"
        if ($null -eq $cred) { Stop-Test "No credential given" }
        try {
            $session = New-PSSession -ComputerName $Server -Credential $cred -Authentication Negotiate
        } catch {
            Write-Info "On the Edge server: Enable-PSRemoting -Force"
            Write-Info "On this machine:    Set-Item WSMan:\localhost\Client\TrustedHosts -Value $Server -Concatenate -Force"
            Write-Info "An administrator other than the built-in Administrator also needs LocalAccountTokenFilterPolicy = 1 on the Edge server."
            Stop-Test "WinRM connection to $Server failed: $($_.Exception.Message)"
        }
        try {
            $info = Invoke-Command -Session $session -ArgumentList $SnapinFilter.ToString() -ScriptBlock {
                param($Filter)
                $s = Get-PSSnapin -Registered | Where-Object ([scriptblock]::Create($Filter)) | Select-Object -First 1
                if (-not $s) { return $null }
                Add-PSSnapin $s.Name
                New-Object PSObject -Property @{
                    Domains       = @(Get-AcceptedDomain | ForEach-Object { "$($_.DomainName)" })
                    Subscriptions = @(Get-EdgeSubscription).Count
                    Name          = $env:COMPUTERNAME
                }
            }
        } finally {
            Remove-PSSession $session
        }
        if ($null -eq $info) { Stop-Test "The Exchange management tools are not installed on $Server" }
        $domains = @($info.Domains)
        $subscriptions = $info.Subscriptions
        $edgeName = $info.Name
    }
    Write-Pass "Connected to $edgeName. Accepted domains: $($domains -join ', ')"
    if ($subscriptions -gt 0) { Write-Info "Subscribed to EdgeSync: accepted and remote domains are only read" }

    # -----------------------------------------------------------------------
    Write-Step "Configuring Terraform for this test"
    $providerDir = (Split-Path -Parent (Resolve-Path -LiteralPath $ProviderPath).Path) -replace '\\', '/'
    $tfrc = Join-Path $env:TEMP "msexchangeserver-edge-test.tfrc"
    Set-Content -Path $tfrc -Encoding ASCII -Value @(
        'provider_installation {',
        '  dev_overrides {',
        "    `"workplace-as-code/msexchangeserver`" = `"$providerDir`"",
        '  }',
        '  direct {}',
        '}'
    )
    $env:TF_CLI_CONFIG_FILE = $tfrc
    $env:TF_IN_AUTOMATION   = "1"
    $env:CHECKPOINT_DISABLE = "1"
    $env:EXCHANGE_ROLE      = "Edge"
    if ($Mode -eq "remote") {
        $env:EXCHANGE_SERVER   = $Server
        $env:EXCHANGE_USERNAME = $Username
        $env:EXCHANGE_PASSWORD = $cred.GetNetworkCredential().Password
    } else {
        Remove-Item Env:EXCHANGE_SERVER, Env:EXCHANGE_USERNAME, Env:EXCHANGE_PASSWORD -ErrorAction SilentlyContinue
    }
    if ($OneShot) { $env:EXCHANGE_BRIDGE_MODE = "oneshot" } else { Remove-Item Env:EXCHANGE_BRIDGE_MODE -ErrorAction SilentlyContinue }
    if ($DebugLog) { $env:TF_LOG = "DEBUG"; $env:TF_LOG_PATH = Join-Path $PSScriptRoot "terraform-debug-$stamp.log" }
    Write-Pass "Development override for the provider: $tfrc"

    # -----------------------------------------------------------------------
    Write-Step "An organization resource must be refused on Edge"
    $refuseDir = Join-Path $env:TEMP "msexchangeserver-edge-refuse-$stamp"
    New-Item -ItemType Directory -Path $refuseDir | Out-Null
    Set-Content -Path (Join-Path $refuseDir "main.tf") -Encoding ASCII -Value @(
        'terraform {',
        '  required_providers {',
        '    msexchangeserver = { source = "workplace-as-code/msexchangeserver" }',
        '  }',
        '}',
        'provider "msexchangeserver" {}',
        'resource "msexchangeserver_owa_mailbox_policy" "refused" {',
        '  name = "TF-CI-Refused"',
        '}'
    )
    Push-Location $refuseDir
    try {
        Assert-Tf (Invoke-Tf init -no-color) "terraform init (refused resource)"
        $r = Invoke-Tf plan -no-color
    } finally {
        Pop-Location
    }
    Remove-TestDir $refuseDir
    if ($r.Code -eq 0 -or -not ($r.Output -match "Not available on an Edge Transport server")) {
        $r.Output | Select-Object -Last 20 | ForEach-Object { Write-Host "       $_" }
        Stop-Test "msexchangeserver_owa_mailbox_policy was not refused with the Edge role"
    }
    Write-Pass "msexchangeserver_owa_mailbox_policy refused at plan time"

    Push-Location $PSScriptRoot
    $pushed = $true
    Remove-Item -Recurse -Force .terraform, .terraform.lock.hcl -ErrorAction SilentlyContinue
    Reset-State
    Assert-Tf (Invoke-Tf init -no-color) "terraform init"
    Write-Pass "terraform init"

    # -----------------------------------------------------------------------
    Write-Step "Plan: data source, transport config import, test rule"
    $r = Invoke-Tf plan -no-color
    Assert-Tf $r "terraform plan"
    $planLine = @($r.Output | Where-Object { $_ -match '^Plan:' }) | Select-Object -First 1
    Write-Pass "terraform plan: $planLine"

    if ($Step -eq "check") {
        Write-Host ""
        Write-Host "Check passed: the provider reaches the Edge server ($Mode)." -ForegroundColor Green
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
    Write-Step "terraform apply (Quarantine rule)"
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color) "terraform apply"
    Write-Pass "Apply"
    Assert-NoChanges "Drift check after apply"

    Write-Step "Update (phase=update: Reject instead of Quarantine)"
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color -var phase=update) "terraform apply (update)"
    Write-Pass "Update"
    Assert-NoChanges "Drift check after update" @("-var", "phase=update")

    # -----------------------------------------------------------------------
    Write-Step "Import of the test rule"
    foreach ($address in $Created.Keys) {
        Assert-Tf (Invoke-Tf state rm -no-color $address) "terraform state rm $address"
        Assert-Tf (Invoke-Tf import -no-color -var phase=update $address $Created[$address]) "Import of $address"
    }
    Write-Pass "$($Created.Count) resource(s) imported"
    Assert-NoChanges "Drift check after import" @("-var", "phase=update")

    # -----------------------------------------------------------------------
    Write-Step "terraform query: generated configuration of the Edge server"
    $minor = [int]($tfVersion.terraform_version -split '\.')[1]
    if ($minor -lt 14) {
        Write-Info "Skipped: terraform query needs Terraform 1.14 or later"
    } else {
        $queryDir = Join-Path $env:TEMP "msexchangeserver-edge-query-$stamp"
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
            Stop-Test "The generated configuration does not match the Edge server: $planLine (see $queryDir)"
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

    Write-Step "Checking that the test rule no longer exists"
    Reset-State
    $left = @(Import-Orphans)
    if ($left.Count -gt 0) {
        Invoke-Tf destroy -auto-approve -no-color @left | Out-Null
        Stop-Test "Still present after destroy (now removed): $($left -join ', ')"
    }
    Reset-State
    Write-Pass "The test rule was deleted (the failed import in the log is the expected proof)"

    Write-Host ""
    Write-Host "All Edge Transport integration tests passed ($Mode)." -ForegroundColor Green
} finally {
    if ($pushed) { Pop-Location }
    Remove-Item Env:EXCHANGE_PASSWORD, Env:EXCHANGE_ROLE -ErrorAction SilentlyContinue
    Write-Host ""
    Write-Host "Log: $Log"
}
