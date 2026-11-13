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
    user@domain. The password is asked for. Use one of -Username, -Integrated
    or -CertificateThumbprint.

.PARAMETER Integrated
    Integrated Windows authentication (auth_method = "integrated"): the
    provider connects as the account running this script, with no password.
    Run it from a session of that account (a service or scheduled task, or
    runas).

.PARAMETER CertificateThumbprint
    Certificate authentication (auth_method = "certificate"): the thumbprint
    of a client certificate in the Personal store of the account running this
    script, mapped to that account in Active Directory. The connection uses
    HTTPS: -Server must match the HTTPS certificate of the Exchange servers.

.PARAMETER ProviderPath
    Path of terraform-provider-msexchangeserver.exe. Defaults to the file next
    to this script.

.PARAMETER OneShot
    Starts one PowerShell process per operation instead of keeping a few open
    (EXCHANGE_BRIDGE_MODE=oneshot). For troubleshooting.

.PARAMETER DebugLog
    Also writes Terraform's debug log (TF_LOG=DEBUG) next to the log file.

.PARAMETER RemoteMailbox
    Identity of an existing remote mailbox (New-RemoteMailbox or
    Enable-RemoteMailbox), read by the remote mailbox data source. Without it,
    that data source is skipped.

.EXAMPLE
    .\run.ps1 -Server exch01.contoso.local -Username CONTOSO\administrator -Step check

.EXAMPLE
    .\run.ps1 -Server exch01.contoso.local -Username CONTOSO\administrator

.EXAMPLE
    .\run.ps1 -Server exch01.contoso.local -Integrated

.EXAMPLE
    .\run.ps1 -Server exch01.contoso.local -CertificateThumbprint 0123456789ABCDEF0123456789ABCDEF01234567
#>
param(
    [Parameter(Mandatory = $true)] [string]$Server,
    [string]$Username,
    [switch]$Integrated,
    [string]$CertificateThumbprint,
    [ValidateSet("check", "all")] [string]$Step = "all",
    [string]$ProviderPath = (Join-Path $PSScriptRoot "terraform-provider-msexchangeserver.exe"),
    [switch]$OneShot,
    [switch]$DebugLog,
    [string]$RemoteMailbox = ""
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
    "msexchangeserver_send_connector.test"          = "TF-CI-Send"
    "msexchangeserver_receive_connector.test"       = "$((($Server -split '\.')[0]).ToUpper())\TF-CI-Receive"
    "msexchangeserver_journal_rule.test"            = "TF-CI-Journal"
    "msexchangeserver_activesync_device_access_rule.test" = "TF-CI-Device (DeviceModel)"
    "msexchangeserver_authentication_policy.test"   = "TF-CI-AuthPolicy"
    "msexchangeserver_address_list.test"            = "TF-CI-AL"
    "msexchangeserver_address_list.child"           = "TF-CI-AL-Child"
    "msexchangeserver_address_list.rooms"           = "TF-CI-Rooms"
    "msexchangeserver_global_address_list.test"     = "TF-CI-GAL"
    "msexchangeserver_offline_address_book.test"    = "TF-CI-OAB"
    "msexchangeserver_address_book_policy.test"     = "TF-CI-ABP"
}
# Organization-wide objects imported by main.tf and never destroyed.
$Keep = @("msexchangeserver_organization_config.this", "msexchangeserver_transport_config.this",
    "msexchangeserver_admin_audit_log_config.this", "msexchangeserver_activesync_organization_settings.this",
    "msexchangeserver_global_address_list.default", "msexchangeserver_offline_address_book.original")
# Per-server objects adopted by main.tf (they exist on every server), imported
# by server name and never destroyed.
$Adopted = @("owa_virtual_directory", "ecp_virtual_directory", "web_services_virtual_directory",
    "activesync_virtual_directory", "oab_virtual_directory", "mapi_virtual_directory", "outlook_anywhere",
    "client_access_service", "mailbox_database", "transport_service", "frontend_transport_service", "pop_settings",
    "imap_settings") | ForEach-Object { "msexchangeserver_$_.test" }
# Adopted objects whose settings the update phase changes: applied again with
# phase=create to put back their original values. restore=true gives IIS, POP,
# IMAP and the default SMTP certificate back to the certificates that had them
# before the test.
$RestoreTargets = @("-target=msexchangeserver_oab_virtual_directory.test", "-target=msexchangeserver_outlook_anywhere.test",
    "-target=msexchangeserver_mailbox_database.test",
    "-target=msexchangeserver_transport_service.test", "-target=msexchangeserver_frontend_transport_service.test",
    "-target=msexchangeserver_pop_settings.test", "-target=msexchangeserver_imap_settings.test",
    "-target=msexchangeserver_admin_audit_log_config.this", "-target=msexchangeserver_organization_config.this",
    "-target=msexchangeserver_activesync_organization_settings.this",
    "-target=msexchangeserver_offline_address_book.original",
    "-target=msexchangeserver_exchange_certificate_services.original",
    "-target=msexchangeserver_exchange_certificate_services.original_iis",
    "-target=msexchangeserver_exchange_certificate_services.original_smtp", "-var", "restore=true")
$restorePending = $false
# Resource types listed by terraform query.
$ListTypes = @("accepted_domain", "remote_domain", "transport_rule", "owa_mailbox_policy",
    "mobile_device_mailbox_policy", "email_address_policy", "retention_policy",
    "retention_policy_tag", "organization_config", "transport_config", "owa_virtual_directory", "ecp_virtual_directory",
    "web_services_virtual_directory", "activesync_virtual_directory", "oab_virtual_directory", "mapi_virtual_directory",
    "outlook_anywhere", "client_access_service", "mailbox_database", "send_connector", "receive_connector", "exchange_certificate",
    "exchange_certificate_services", "transport_service", "frontend_transport_service", "pop_settings", "imap_settings",
    "admin_audit_log_config", "journal_rule", "activesync_organization_settings", "activesync_device_access_rule",
    "authentication_policy", "address_list", "global_address_list", "offline_address_book", "address_book_policy")

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

# Get-StateAttr returns an attribute of a resource in the state.
function Get-StateAttr([string]$Address, [string]$Name) {
    # Not through Invoke-Tf: the whole state would go to the log.
    $json = @(& terraform show -json -no-color 2>$null)
    if ($LASTEXITCODE -ne 0) { Stop-Test "terraform show failed (exit $LASTEXITCODE)" }
    # Windows PowerShell 5.1: assign the parsed JSON first, then use it.
    $state = ($json -join "`n") | ConvertFrom-Json
    $resource = @($state.values.root_module.resources | Where-Object { $_.address -eq $Address }) | Select-Object -First 1
    if ($null -eq $resource) { Stop-Test "$Address is not in the state" }
    $resource.values.$Name
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
    $authText = if ($Integrated) { "-Integrated (as $account)" } elseif ($CertificateThumbprint) { "-CertificateThumbprint $CertificateThumbprint (as $account)" } else { "-Username $Username" }
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
    if (@($Username, $Integrated.IsPresent, $CertificateThumbprint | Where-Object { $_ }).Count -ne 1) {
        Stop-Test "Use one of -Username (password asked for), -Integrated (the account running this script) or -CertificateThumbprint (a certificate of that account)"
    }
    if ($Server -notmatch '\.' -or $Server -match '^\d+(\.\d+){3}$') {
        Stop-Test "-Server must be the FQDN of the Exchange server (Kerberos does not work with an IP address or a short name)"
    }

    # -----------------------------------------------------------------------
    Write-Step "Test certificate"
    # Self-signed, made once per account and reused while it stays valid: a
    # certificate left on the server by an interrupted run then has the
    # thumbprint of the next run, whose pre-cleanup removes it. Its key is in a
    # CryptoAPI provider: Exchange refuses a CNG key, the default of
    # New-SelfSignedCertificate, for the default SMTP certificate.
    $certDir = Join-Path $env:LOCALAPPDATA "msexchangeserver-test"
    $pfxPath = Join-Path $certDir "tf-ci-cert-capi.pfx"
    $pfxPasswordPath = Join-Path $certDir "tf-ci-cert-capi.txt"
    $pfxData = $null
    if ((Test-Path -LiteralPath $pfxPath) -and (Test-Path -LiteralPath $pfxPasswordPath)) {
        $pfxPassword = (Get-Content -Raw -LiteralPath $pfxPasswordPath).Trim()
        $pfxData = Get-PfxData -FilePath $pfxPath -Password (ConvertTo-SecureString $pfxPassword -AsPlainText -Force)
        if ($pfxData.EndEntityCertificates[0].NotAfter -lt (Get-Date).AddDays(30)) { $pfxData = $null }
    }
    if ($null -eq $pfxData) {
        New-Item -ItemType Directory -Force -Path $certDir | Out-Null
        $pfxPassword = [guid]::NewGuid().ToString()
        $selfSigned = New-SelfSignedCertificate -Subject "CN=tf-ci-cert.example" -DnsName "tf-ci-cert.example" `
            -CertStoreLocation Cert:\CurrentUser\My -KeyExportPolicy Exportable -NotAfter (Get-Date).AddYears(1) `
            -Provider "Microsoft Enhanced RSA and AES Cryptographic Provider" -KeySpec KeyExchange
        try {
            Export-PfxCertificate -Cert $selfSigned -FilePath $pfxPath -Password (ConvertTo-SecureString $pfxPassword -AsPlainText -Force) | Out-Null
        } finally {
            Remove-Item -LiteralPath "Cert:\CurrentUser\My\$($selfSigned.Thumbprint)" -DeleteKey
        }
        Set-Content -LiteralPath $pfxPasswordPath -Value $pfxPassword -NoNewline -Encoding ASCII
        $pfxData = Get-PfxData -FilePath $pfxPath -Password (ConvertTo-SecureString $pfxPassword -AsPlainText -Force)
        Write-Info "New test certificate in $pfxPath"
    }
    $testCertificate = $pfxData.EndEntityCertificates[0].Thumbprint
    $Created["msexchangeserver_exchange_certificate.test"] = "$Server\$testCertificate"
    $Created["msexchangeserver_exchange_certificate_services.test"] = "$Server\$testCertificate"
    Write-Pass "Test certificate $testCertificate (valid until $($pfxData.EndEntityCertificates[0].NotAfter.ToString('yyyy-MM-dd')))"

    # -----------------------------------------------------------------------
    $scheme = if ($CertificateThumbprint) { "https" } else { "http" }
    Write-Step "Connection to ${scheme}://$Server/PowerShell/"
    # Integrated: no credential, New-PSSession uses the Kerberos ticket of this session
    $credential = @{ Authentication = "Kerberos" }
    if ($Integrated) {
        Write-Info "Integrated authentication as $account"
    } elseif ($CertificateThumbprint) {
        Write-Info "Certificate authentication as $account"
        $credential = @{ CertificateThumbprint = $CertificateThumbprint }
    } else {
        $cred = Get-Credential -UserName $Username -Message "Exchange Server account of the provider"
        if ($null -eq $cred) { Stop-Test "No credential given" }
        $credential.Credential = $cred
    }
    $session = New-PSSession -ConfigurationName Microsoft.Exchange -ConnectionUri "${scheme}://$Server/PowerShell/" @credential
    try {
        # The Exchange endpoint runs remote commands without the PowerShell
        # language (NoLanguage mode): only plain cmdlet calls, results processed here.
        $domains = @(Invoke-Command -Session $session -ScriptBlock { Get-AcceptedDomain })
        $databases = @(Invoke-Command -Session $session -ScriptBlock { Get-MailboxDatabase })
        # Selected on the server: the certificates would otherwise come back
        # as X509Certificate2 objects, without their services.
        $certificates = @(Invoke-Command -Session $session -ScriptBlock { Get-ExchangeCertificate | Select-Object Thumbprint, Services, Subject, IsSelfSigned })
        $transport = @(Invoke-Command -Session $session -ScriptBlock { Get-TransportService | Select-Object Name, InternalTransportCertificateThumbprint })
        $oabs = @(Invoke-Command -Session $session -ScriptBlock { Get-OfflineAddressBook | Select-Object Name, IsDefault })
        # Only to show the version: the provider's role group has no Get-ExchangeServer.
        $servers = @(try { Invoke-Command -Session $session -ScriptBlock { Get-ExchangeServer } -ErrorAction Stop } catch { })
        # The cmdlets the account can run, for the preflight below.
        $commands = @(Invoke-Command -Session $session -ScriptBlock { Get-Command } | ForEach-Object { [string]$_.Name })
    } finally {
        Remove-PSSession $session
    }
    $default = @($domains | Where-Object { "$($_.Default)" -eq "True" }) | Select-Object -First 1
    if ($null -eq $default) { Stop-Test "No default accepted domain found" }
    Write-Pass "Connected. Accepted domains: $(($domains | ForEach-Object { "$($_.DomainName)" }) -join ', ')"
    if ($Step -eq "all") {
        # Preflight: the account can run every cmdlet of the provider
        # (cmdlets.txt, kept equal to $ProviderCmdlets of
        # tools\new_role_group.ps1 by tools\tools_test.go), before any change.
        $wanted = @(Get-Content -Path (Join-Path $PSScriptRoot "cmdlets.txt") | Where-Object { $_ })
        $missing = @($wanted | Where-Object { $commands -notcontains $_ })
        if ($missing.Count -gt 0) {
            Stop-Test ("The account cannot run these cmdlets of the provider: $($missing -join ', '). Run " +
                "tools\new_role_group.ps1 -Apply again; a cmdlet in no role does not exist in this Exchange version.")
        }
        Write-Pass "Preflight: the account can run the $($wanted.Count) cmdlets of the provider"
    }
    # The mailbox database adopted by the test: one of the tested server, and its
    # deleted item retention, which the update phase changes and run.ps1 restores.
    $database = @($databases | Where-Object { "$($_.Server)" -eq ($Server -split '\.')[0] }) + @($databases) | Select-Object -First 1
    if ($null -eq $database) { Stop-Test "No mailbox database found" }
    $retentionDays = [int](("$($database.DeletedItemRetention)" -split '\.')[0])
    Write-Pass "Mailbox database: $($database.Name) (deleted item retention: $retentionDays days)"
    # The certificate enabled for POP and IMAP, which the test moves to its own
    # certificate and back.
    $original = @($certificates | Where-Object {
        # [string]: "$x" gives the number of a deserialized enum, not its names
        [string]$_.Services -match 'POP' -and [string]$_.Services -match 'IMAP' -and "$($_.Thumbprint)" -ne $testCertificate
    }) | Select-Object -First 1
    if ($null -eq $original) { Stop-Test "No certificate of $Server is enabled for POP and IMAP besides the test one: enable them on its certificate first" }
    Write-Pass "Certificate for POP and IMAP: $($original.Thumbprint) ($($original.Subject))"
    # The default SMTP certificate, which the update phase replaces with the
    # test one and run.ps1 puts back.
    $originalSmtp = [string](@($transport | Where-Object { "$($_.Name)" -eq ($Server -split '\.')[0] }) | Select-Object -First 1).InternalTransportCertificateThumbprint
    if (-not $originalSmtp -or $originalSmtp -eq $testCertificate) { Stop-Test "No default SMTP certificate found for $Server besides the test one: enable SMTP on its certificate first" }
    Write-Pass "Default SMTP certificate: $originalSmtp"
    # The default offline address book, which the update phase replaces with the
    # test one and run.ps1 makes the default again.
    $originalOab = @($oabs | Where-Object { "$($_.IsDefault)" -eq "True" -and "$($_.Name)" -ne "TF-CI-OAB" }) | Select-Object -First 1
    if ($null -eq $originalOab) { Stop-Test "No default offline address book found besides TF-CI-OAB: make the original one the default first" }
    Write-Pass "Default offline address book: $($originalOab.Name)"
    # The HTTPS certificate of the server, which the update phase replaces with
    # the test one: the Exchange servers' self-signed certificate is enabled for
    # IIS too, for the back-end site, so prefer one issued by a CA.
    $originalIis = @($certificates | Where-Object { [string]$_.Services -match 'IIS' -and "$($_.Thumbprint)" -ne $testCertificate } |
        Sort-Object { [bool]$_.IsSelfSigned }) | Select-Object -First 1
    if ($null -eq $originalIis) { Stop-Test "No certificate of $Server is enabled for IIS besides the test one" }
    Write-Pass "Certificate for IIS: $($originalIis.Thumbprint) ($($originalIis.Subject))"
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
    Remove-Item Env:EXCHANGE_CERTIFICATE_THUMBPRINT -ErrorAction SilentlyContinue
    if ($Integrated) {
        $env:EXCHANGE_AUTH_METHOD = "integrated"
        Remove-Item Env:EXCHANGE_USERNAME, Env:EXCHANGE_PASSWORD -ErrorAction SilentlyContinue
    } elseif ($CertificateThumbprint) {
        $env:EXCHANGE_AUTH_METHOD = "certificate"
        $env:EXCHANGE_CERTIFICATE_THUMBPRINT = $CertificateThumbprint
        Remove-Item Env:EXCHANGE_USERNAME, Env:EXCHANGE_PASSWORD -ErrorAction SilentlyContinue
    } else {
        $env:EXCHANGE_AUTH_METHOD = "password"
        $env:EXCHANGE_USERNAME    = $Username
        $env:EXCHANGE_PASSWORD    = $cred.GetNetworkCredential().Password
    }
    $env:TF_VAR_default_domain = "$($default.Name)"
    $env:TF_VAR_test_remote_mailbox = $RemoteMailbox
    $env:TF_VAR_exchange_server = $Server
    $env:TF_VAR_test_database = "$($database.Name)"
    $env:TF_VAR_test_database_retention_days = "$retentionDays"
    $env:TF_VAR_test_certificate = $testCertificate
    $env:TF_VAR_test_pfx_base64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes($pfxPath))
    $env:TF_VAR_test_pfx_password = $pfxPassword
    $env:TF_VAR_original_certificate = "$($original.Thumbprint)"
    $env:TF_VAR_original_iis_certificate = "$($originalIis.Thumbprint)"
    $env:TF_VAR_original_smtp_certificate = $originalSmtp
    $env:TF_VAR_original_default_oab = "$($originalOab.Name)"
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
        Write-Host "Run the full cycle with: .\run.ps1 -Server $Server $($authText -replace ' \(as .*\)$', '')"
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
    # From here on, the adopted objects and the POP service hold test values:
    # the finally block puts the original ones back if the run stops before
    # the destroy step.
    $restorePending = $true
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color) "terraform apply"
    Write-Pass "Apply"
    Assert-NoChanges "Drift check after apply"

    # Authentication settings read from IIS: the update writes the opposite of
    # the original values, the restore writes the originals back.
    $oabBasic = [bool](Get-StateAttr "msexchangeserver_oab_virtual_directory.test" "basic_authentication")
    $oaMethod = [string](Get-StateAttr "msexchangeserver_outlook_anywhere.test" "internal_client_authentication_method")
    if ($oaMethod -eq "") { Stop-Test "Outlook Anywhere has no internal client authentication method in the state: IIS was not read" }
    $oaOther = if ($oaMethod -eq "Negotiate") { "Ntlm" } else { "Negotiate" }
    Write-Info "Original values: OAB basic_authentication=$oabBasic, Outlook Anywhere internal method=$oaMethod"
    $AuthUpdate = @("-var", "oab_basic_authentication=$((-not $oabBasic).ToString().ToLower())",
        "-var", "oa_internal_client_authentication_method=$oaOther")
    $RestoreTargets += @("-var", "oab_basic_authentication=$($oabBasic.ToString().ToLower())",
        "-var", "oa_internal_client_authentication_method=$oaMethod")

    # Log, POP3, IMAP4 and audit settings: the update raises each original
    # value by one, which removes no log and no audit entry. A log without
    # age limit (0) stays unmanaged: 1 would remove its older files.
    $originals = [ordered]@{
        transport_service          = Get-StateAttr "msexchangeserver_transport_service.test" "connectivity_log_max_age_days"
        frontend_transport_service = Get-StateAttr "msexchangeserver_frontend_transport_service.test" "connectivity_log_max_age_days"
        pop_settings               = Get-StateAttr "msexchangeserver_pop_settings.test" "max_connections_per_user"
        imap_settings              = Get-StateAttr "msexchangeserver_imap_settings.test" "max_connections_per_user"
        admin_audit_log_config     = Get-StateAttr "msexchangeserver_admin_audit_log_config.this" "admin_audit_log_age_limit_days"
    }
    $updated = @(); $restored = @()
    foreach ($key in $originals.Keys) {
        $value = [long]$originals[$key]
        if ($value -lt 1 -or $value -ge 24855) { Write-Info "Original $key value $value left unchanged"; continue }
        $updated += "$key=$($value + 1)"
        $restored += "$key=$value"
    }
    Write-Info "Original values: $($restored -join ', ')"
    $AuthUpdate += @("-var", "settings={$($updated -join ',')}")
    $RestoreTargets += @("-var", "settings={$($restored -join ',')}")
    # Text settings: the test policy becomes the organization default and the
    # ActiveSync quarantine text changes; the restore writes the originals.
    $defaultPolicy = [string](Get-StateAttr "msexchangeserver_organization_config.this" "default_authentication_policy")
    $userMailInsert = [string](Get-StateAttr "msexchangeserver_activesync_organization_settings.this" "user_mail_insert")
    Write-Info "Original values: default authentication policy '$defaultPolicy', ActiveSync quarantine text '$userMailInsert'"
    # In variable files: Windows PowerShell 5.1 drops the quotes inside the
    # arguments of a native command such as terraform.exe.
    $textsUpdate = Join-Path $PSScriptRoot "texts-update.tfvars.json"
    $textsRestore = Join-Path $PSScriptRoot "texts-restore.tfvars.json"
    ConvertTo-Json -Compress -InputObject @{ texts = @{ default_authentication_policy = "TF-CI-AuthPolicy"
        activesync_user_mail_insert = "[TF-CI-TEST] quarantined" } } | Set-Content -LiteralPath $textsUpdate -Encoding ASCII
    ConvertTo-Json -Compress -InputObject @{ texts = @{ default_authentication_policy = $defaultPolicy
        activesync_user_mail_insert = $userMailInsert } } | Set-Content -LiteralPath $textsRestore -Encoding ASCII
    $AuthUpdate += @("-var-file=$textsUpdate")
    $RestoreTargets += @("-var-file=$textsRestore")

    Write-Step "Update of every test object (phase=update)"
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color -var phase=update @AuthUpdate) "terraform apply (update)"
    Write-Pass "Update"
    Assert-NoChanges "Drift check after update" (@("-var", "phase=update") + $AuthUpdate)

    # -----------------------------------------------------------------------
    Write-Step "Import of every test object"
    foreach ($address in $Created.Keys) {
        Assert-Tf (Invoke-Tf state rm -no-color $address) "terraform state rm $address"
        Assert-Tf (Invoke-Tf import -no-color -var phase=update $address $Created[$address]) "Import of $address"
    }
    foreach ($address in $Adopted) {
        Assert-Tf (Invoke-Tf state rm -no-color $address) "terraform state rm $address"
        $id = if ($address -like "*mailbox_database*") { "$($database.Name)" } else { $Server }
        Assert-Tf (Invoke-Tf import -no-color -var phase=update $address $id) "Import of $address"
    }
    Write-Pass "$($Created.Count + $Adopted.Count) resources imported"
    # apply_to_recipients is a provider setting that Exchange does not store: an
    # imported policy has none, so the first apply only records it. It applies
    # nothing, since no setting of the policy changes.
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color -var phase=update "-target=msexchangeserver_email_address_policy.test") "Recording apply_to_recipients after import"
    # An imported filter holds Exchange's text, with parentheses: the first apply
    # writes the same filter in the configured form.
    $filters = @("address_list.test", "address_list.child", "address_list.rooms", "global_address_list.test") |
        ForEach-Object { "-target=msexchangeserver_$_" }
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color -var phase=update @filters) "Recording the filters after import"
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
    # Adopted objects stay: put back what the update phase changed first.
    Assert-Tf (Invoke-Tf apply -auto-approve -no-color @RestoreTargets) "Restoring the adopted objects"
    Assert-NoChanges "OAB and Outlook Anywhere settings, database retention and certificates restored" $RestoreTargets
    $restorePending = $false
    foreach ($address in @($Keep) + $Adopted + @("original", "original_iis", "original_smtp" | ForEach-Object { "msexchangeserver_exchange_certificate_services.$_[0]" })) { Assert-Tf (Invoke-Tf state rm -no-color $address) "terraform state rm $address" }
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
    Write-Host "  Imported read-only: organization_config, transport_config; imported and restored: admin_audit_log_config"
    Write-Host "  Data sources: accepted_domain, accepted_domains, organization_config"
} finally {
    if ($restorePending) {
        Write-Host ""
        Write-Host "Putting back the original OAB and Outlook Anywhere settings, database retention and certificates..." -ForegroundColor Yellow
        $r = Invoke-Tf apply -auto-approve -no-color @RestoreTargets
        if ($r.Code -ne 0) {
            Write-Host "  WARN Could not put them back: apply test\main.tf with phase=create and the targets of `$RestoreTargets" -ForegroundColor Red
        }
    }
    if ($pushed) { Pop-Location }
    # The other test scripts of this window (edge, transport) do not set the method
    Remove-Item Env:EXCHANGE_PASSWORD, Env:EXCHANGE_AUTH_METHOD -ErrorAction SilentlyContinue
    Write-Host ""
    Write-Host "Log: $Log"
}
