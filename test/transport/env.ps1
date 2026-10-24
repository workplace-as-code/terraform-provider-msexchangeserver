<#
.SYNOPSIS
    Prepares this PowerShell window to run terraform by hand in this folder,
    against the provider build of the test kit. Dot-source it:

        . .\env.ps1 -Server exch01.contoso.local -Username CONTOSO\administrator
        . .\env.ps1 -Role Edge                  # on an Edge server, locally
        . .\env.ps1 -Role Edge -Server edge01.contoso.com -Username EDGE01\Administrator
#>
param(
    [string]$Server = "",
    [string]$Username = "",
    [ValidateSet("Mailbox", "Edge")] [string]$Role = "Mailbox",
    [string]$ProviderPath = (Join-Path (Split-Path -Parent $PSScriptRoot) "terraform-provider-msexchangeserver.exe")
)

if (-not (Test-Path -LiteralPath $ProviderPath -PathType Leaf)) { throw "Provider not found: $ProviderPath" }
if ($Role -eq "Mailbox" -and -not $Server) { throw "-Server is required with the Mailbox role" }
if ($Server -and -not $Username) { throw "-Username is required with -Server" }

$providerDir = (Split-Path -Parent (Resolve-Path -LiteralPath $ProviderPath).Path) -replace '\\', '/'
$tfrc = Join-Path $env:TEMP "msexchangeserver-manual-test.tfrc"
Set-Content -Path $tfrc -Encoding ASCII -Value @(
    'provider_installation {',
    '  dev_overrides {',
    "    `"workplace-as-code/msexchangeserver`" = `"$providerDir`"",
    '  }',
    '  direct {}',
    '}'
)
$env:TF_CLI_CONFIG_FILE = $tfrc
$env:CHECKPOINT_DISABLE = "1"
$env:EXCHANGE_ROLE      = $Role
if ($Server) {
    $cred = Get-Credential -UserName $Username -Message "Exchange account ($Role)"
    $env:EXCHANGE_SERVER   = $Server
    $env:EXCHANGE_USERNAME = $Username
    $env:EXCHANGE_PASSWORD = $cred.GetNetworkCredential().Password
} else {
    Remove-Item Env:EXCHANGE_SERVER, Env:EXCHANGE_USERNAME, Env:EXCHANGE_PASSWORD -ErrorAction SilentlyContinue
}
Write-Host "Ready: terraform uses $ProviderPath ($Role$(if ($Server) { ", $Server" } else { ", local" }))." -ForegroundColor Green
Write-Host "Close this window when you are done: it holds the password in EXCHANGE_PASSWORD."
