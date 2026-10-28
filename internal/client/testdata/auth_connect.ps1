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
#   -Case servers           : two servers, the first one does not answer
#   -Case servers-auth      : two servers, Kerberos fails on the first one
#   -Case servers-down      : two servers, neither answers
#   -Case servers-maint     : two servers, the health page of the first one fails
#   -Case servers-noprobe   : two servers, no health page answers
#   -Case certificate       : EXCHANGE_BRIDGE_AUTH_METHOD=certificate and a thumbprint
#   -Case certificate-missing : the same, and the certificate is not in the store
param([string]$Script, [string]$Case)

$ErrorActionPreference = "Stop"

# Functions take precedence over cmdlets of the same name.
function New-PSSessionOption { "options" }

# auth.ps1 defines Test-ExchangeHealth in its own scope; an alias takes
# precedence over any function, so this fake answers instead.
function Test-FakeExchangeHealth([string]$Name) {
    $global:Probes += @($Name)
    switch ($Case) {
        "servers-maint"   { return $Name -notlike "exch01*" }
        "servers-noprobe" { return $false }
        default           { return $true }
    }
}
Set-Alias -Name Test-ExchangeHealth -Value Test-FakeExchangeHealth -Scope Global
# The same for the lookup of the client certificate in the Windows store.
function Find-FakeClientCertificate([string]$Thumbprint) {
    if ($Case -eq "certificate-missing") { return $null }
    return "certificate $Thumbprint"
}
Set-Alias -Name Find-ClientCertificate -Value Find-FakeClientCertificate -Scope Global
function New-PSSession {
    param($ConfigurationName, $ConnectionUri, $Authentication, $Credential, $SessionOption, $CertificateThumbprint)
    $global:Attempts += @("$ConnectionUri")
    if ($Case -eq "servers-auth") { throw "WinRM cannot process the request: Kerberos error" }
    if (($Case -eq "servers" -and "$ConnectionUri" -like "*exch01*") -or $Case -eq "servers-down") {
        throw "Connecting to remote server failed: the WinRM client cannot complete the operation within the time specified"
    }
    if ($Case -eq "integrated-fails") { throw "WinRM cannot process the request: Kerberos error" }
    if ($Case -eq "integrated-503") { throw "Connecting to remote server exch01 failed with the following error message : $([char]0x73E0)$([char]0x2C4C) For more information, see the about_Remote_Troubleshooting Help topic." }
    $global:Connected = [ordered]@{
        ConnectionUri  = "$ConnectionUri"
        Authentication = "$Authentication"
        Credential     = $PSBoundParameters.ContainsKey("Credential")
        CertificateThumbprint = "$CertificateThumbprint"
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
$server = "exch01.contoso.local"
switch ($Case) {
    "password"         { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "password"; $env:EXCHANGE_BRIDGE_PASSWORD = "pw"; $username = "CONTOSO\svc" }
    { $_ -like "servers*" } {
        $env:EXCHANGE_BRIDGE_AUTH_METHOD = "password"; $env:EXCHANGE_BRIDGE_PASSWORD = "pw"; $username = "CONTOSO\svc"
        $server = "exch01.contoso.local, exch02.contoso.local"
    }
    "password-nouser"  { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "password"; $env:EXCHANGE_BRIDGE_PASSWORD = "pw" }
    { $_ -like "integrated*" } { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "integrated" }
    { $_ -like "certificate*" } { $env:EXCHANGE_BRIDGE_AUTH_METHOD = "certificate"; $env:EXCHANGE_BRIDGE_CERTIFICATE_THUMBPRINT = "0123456789ABCDEF0123456789ABCDEF01234567" }
    default { throw "unknown case $Case" }
}
$env:EXCHANGE_BRIDGE_DOMAIN_CONTROLLER = if ($Case -like "integrated-dc*") { "dc01.contoso.local" } else { "" }

$global:Connected = $null
$global:Attempts = @()
$global:Probes = @()
$errorText = ""
try {
    . $Script -Server $server -Username $username
} catch {
    $errorText = "$_"
}
ConvertTo-Json -Compress -InputObject @{ connected = $global:Connected; error = $errorText; user = [Environment]::UserName; attempts = @($global:Attempts); probes = @($global:Probes) }
