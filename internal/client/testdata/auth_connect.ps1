# auth_connect.ps1 - runs auth.ps1 against fake remoting cmdlets and prints, as
# JSON, how it called New-PSSession, or the error it threw (run by
# scripts_test.go).
#   -Case password          : username and EXCHANGE_BRIDGE_PASSWORD
#   -Case password-nouser   : password method without a username
#   -Case integrated        : EXCHANGE_BRIDGE_AUTH_METHOD=integrated, no username
#   -Case integrated-fails  : the same, and New-PSSession fails (Kerberos)
#   -Case integrated-503    : the same, and Exchange answers with an HTTP error page
#   -Case integrated-dc     : integrated, with EXCHANGE_BRIDGE_DOMAIN_CONTROLLER
#   -Case integrated-dc-fails : the same, and Exchange refuses the domain controller
param([string]$Script, [string]$Case)

$ErrorActionPreference = "Stop"

# Functions take precedence over cmdlets of the same name.
function New-PSSessionOption { "options" }
function New-PSSession {
    param($ConfigurationName, $ConnectionUri, $Authentication, $Credential, $SessionOption)
    if ($Case -eq "integrated-fails") { throw "WinRM cannot process the request: Kerberos error" }
    if ($Case -eq "integrated-503") { throw "Connecting to remote server exch01 failed with the following error message : $([char]0x73E0)$([char]0x2C4C) For more information, see the about_Remote_Troubleshooting Help topic." }
    $global:Connected = [ordered]@{
        ConnectionUri  = "$ConnectionUri"
        Authentication = "$Authentication"
        Credential     = $PSBoundParameters.ContainsKey("Credential")
    }
    "session"
}
function Import-PSSession { "module" }
function Import-Module { }
function Set-ADServerSettings {
    param($PreferredServer)
    if ($Case -eq "integrated-dc-fails") { throw "error 0x51 ... check the suitability of server $PreferredServer" }
    $global:Connected.PreferredServer = "$PreferredServer"
}

$username = ""
switch ($Case) {
    "password"         { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "password"; $env:EXCHANGE_BRIDGE_PASSWORD = "pw"; $username = "CONTOSO\svc" }
    "password-nouser"  { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "password"; $env:EXCHANGE_BRIDGE_PASSWORD = "pw" }
    { $_ -like "integrated*" } { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "integrated" }
    default { throw "unknown case $Case" }
}
$env:EXCHANGE_BRIDGE_DOMAIN_CONTROLLER = if ($Case -like "integrated-dc*") { "dc01.contoso.local" } else { "" }

$global:Connected = $null
$errorText = ""
try {
    . $Script -Server exch01.contoso.local -Username $username
} catch {
    $errorText = "$_"
}
ConvertTo-Json -Compress -InputObject @{ connected = $global:Connected; error = $errorText; user = [Environment]::UserName }
