# auth_connect.ps1 - runs auth.ps1 against fake remoting cmdlets and prints, as
# JSON, how it called New-PSSession, or the error it threw (run by
# scripts_test.go).
#   -Case password          : username and EXCHANGE_BRIDGE_PASSWORD
#   -Case password-nouser   : password method without a username
#   -Case integrated        : EXCHANGE_BRIDGE_AUTH_METHOD=integrated, no username
#   -Case integrated-fails  : the same, and New-PSSession fails (Kerberos)
param([string]$Script, [string]$Case)

$ErrorActionPreference = "Stop"

# Functions take precedence over cmdlets of the same name.
function New-PSSessionOption { "options" }
function New-PSSession {
    param($ConfigurationName, $ConnectionUri, $Authentication, $Credential, $SessionOption)
    if ($Case -eq "integrated-fails") { throw "WinRM cannot process the request: Kerberos error" }
    $global:Connected = [ordered]@{
        ConnectionUri  = "$ConnectionUri"
        Authentication = "$Authentication"
        Credential     = $PSBoundParameters.ContainsKey("Credential")
    }
    "session"
}
function Import-PSSession { "module" }
function Import-Module { }

$username = ""
switch ($Case) {
    "password"         { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "password"; $env:EXCHANGE_BRIDGE_PASSWORD = "pw"; $username = "CONTOSO\svc" }
    "password-nouser"  { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "password"; $env:EXCHANGE_BRIDGE_PASSWORD = "pw" }
    { $_ -like "integrated*" } { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "integrated" }
    default { throw "unknown case $Case" }
}

$global:Connected = $null
$errorText = ""
try {
    . $Script -Server exch01.contoso.local -Username $username
} catch {
    $errorText = "$_"
}
ConvertTo-Json -Compress -InputObject @{ connected = $global:Connected; error = $errorText; user = [Environment]::UserName }
