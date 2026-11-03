# exchange_certificate.ps1 - Exchange certificates of a server: read, import
# from a PFX, enable for services, remove.
# Called by the Terraform provider via the Go client bridge.
#
#   -Identity      "<server>\<thumbprint>"
#   -PfxData       Import: the PFX file in base64
#   -PfxPassword   Import: its password ("" when it has none)
#   -Services      EnableServices: "IIS,SMTP"; only the services the
#                  certificate does not carry yet are enabled
#
# Output: the certificate as JSON; Get on a missing certificate gives an empty
# stdout.

param(
    [string]$Server                = "",
    [string]$Username              = "",
    [string]$Password              = "",

    [Parameter(Mandatory)] [ValidateSet("Get","Import","EnableServices","Delete")] [string]$Action,

    [Parameter(Mandatory)] [string]$Identity,
    [string]$PfxData              = "",
    [string]$PfxPassword          = "",
    [string]$FriendlyName         = "",
    [string]$PrivateKeyExportable = "",
    [string]$Services             = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot/auth.ps1" -Server $Server -Username $Username -Password $Password
. "$PSScriptRoot/common.ps1"

# $Server is the connection parameter of auth.ps1: the server that holds the
# certificate gets its own name.
$certServer, $thumbprint = $Identity -split '\\', 2
# Both values go into a command line run on the server (Get-BridgeCertificate).
if ($certServer -notmatch '^[A-Za-z0-9][A-Za-z0-9.-]*$' -or $thumbprint -notmatch '^[0-9A-Fa-f]{40}$') {
    [Console]::Error.WriteLine("Identity must be '<server>\<thumbprint>', got '$Identity'")
    exit 1
}

# Services that a removal would break: Exchange removes the certificate anyway.
# SMTP is not one of them: Exchange adds it to certificates on its own and
# never takes it away.
$BoundServices = @("IIS", "POP", "IMAP")

function Get-BridgeServices($Certificate) {
    @("$(Get-BridgeProp $Certificate 'Services' '')" -split ',' |
        ForEach-Object { $_.Trim() } |
        Where-Object { $_ -and $_ -ne "None" })
}

function ConvertTo-BridgeDate($Value) {
    if ($null -eq $Value) { return "" }
    ([datetime]$Value).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ", [Globalization.CultureInfo]::InvariantCulture)
}

# Get-BridgeCertificate reads the certificate, or $null when it does not exist.
# Through the remote session, PowerShell turns an Exchange certificate back
# into a plain X509Certificate2, and the server sends none of its Exchange
# properties (Services, CertificateDomains, Status): Select-Object runs on the
# server instead. The endpoint's NoLanguage mode accepts a pipeline of literal
# values only, hence the command built as a string from checked values.
$CertificateProperties = "Thumbprint", "FriendlyName", "Subject", "Issuer", "CertificateDomains", "NotBefore",
    "NotAfter", "Status", "IsSelfSigned", "HasPrivateKey", "PrivateKeyExportable", "Services"
function Get-BridgeCertificate {
    $session = @(Get-PSSession | Where-Object { $_.ConfigurationName -eq "Microsoft.Exchange" -and $_.State -eq "Opened" }) |
        Select-Object -First 1
    try {
        if ($null -eq $session) {
            return Get-ExchangeCertificate -Server $certServer -Thumbprint $thumbprint -ErrorAction Stop
        }
        $command = "Get-ExchangeCertificate -Server '$certServer' -Thumbprint '$thumbprint' | Select-Object $($CertificateProperties -join ', ')"
        Invoke-Command -Session $session -ScriptBlock ([scriptblock]::Create($command)) -ErrorAction Stop
    } catch {
        if (Test-BridgeNotFound $_.Exception.Message) { return $null }
        throw
    }
}

# Read-BridgeCertificate reads a certificate that must exist, after a write.
function Read-BridgeCertificate {
    Invoke-BridgeRetry {
        $cert = Get-BridgeCertificate
        if ($null -eq $cert) { throw "The certificate $thumbprint was not found on $certServer after the change." }
        $cert
    }
}

function ConvertTo-BridgeCertificate($Certificate) {
    @{
        Identity             = "$certServer\$($Certificate.Thumbprint)"
        Thumbprint           = [string]$Certificate.Thumbprint
        FriendlyName         = [string](Get-BridgeProp $Certificate "FriendlyName" "")
        Subject              = [string](Get-BridgeProp $Certificate "Subject" "")
        Issuer               = [string](Get-BridgeProp $Certificate "Issuer" "")
        CertificateDomains   = @(ConvertTo-BridgeStrings (Get-BridgeProp $Certificate "CertificateDomains" @()))
        NotBefore            = ConvertTo-BridgeDate (Get-BridgeProp $Certificate "NotBefore")
        NotAfter             = ConvertTo-BridgeDate (Get-BridgeProp $Certificate "NotAfter")
        Status               = [string](Get-BridgeProp $Certificate "Status" "")
        IsSelfSigned         = [bool](Get-BridgeProp $Certificate "IsSelfSigned" $false)
        HasPrivateKey        = [bool](Get-BridgeProp $Certificate "HasPrivateKey" $false)
        PrivateKeyExportable = [bool](Get-BridgeProp $Certificate "PrivateKeyExportable" $false)
        Services             = @(Get-BridgeServices $Certificate)
    } | ConvertTo-Json -Compress
}

# Get-PfxThumbprint reads the thumbprint of the certificate with a private key
# in the PFX, without importing anything: a wrong file or password fails here,
# before Exchange is called. EphemeralKeySet (32) keeps the key in memory.
function Get-PfxThumbprint([byte[]]$Bytes) {
    $certs = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2Collection
    try {
        $certs.Import($Bytes, $PfxPassword, [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]32)
    } catch {
        throw "The PFX could not be read: check pfx_base64_wo (base64 of the .pfx file) and pfx_password_wo."
    }
    $withKey = @($certs | Where-Object { $_.HasPrivateKey })
    if ($withKey.Count -ne 1) { throw "The PFX must hold exactly one certificate with a private key; it holds $($withKey.Count)." }
    $withKey[0].Thumbprint
}

try {
    switch ($Action) {
        "Get" {
            $cert = Get-BridgeCertificate
            if ($null -eq $cert) { exit 0 }
            ConvertTo-BridgeCertificate $cert
        }

        "Import" {
            $bytes = [Convert]::FromBase64String($PfxData)
            $pfxThumbprint = Get-PfxThumbprint $bytes
            if ($pfxThumbprint -ne $thumbprint) {
                throw "The PFX holds the certificate $pfxThumbprint, not $thumbprint."
            }
            if ($null -ne (Get-BridgeCertificate)) {
                throw "The certificate $thumbprint already exists on $certServer. Import it into the Terraform state instead (terraform import, ID '$Identity')."
            }
            $params = @{ Server = $certServer; FileData = $bytes }
            if ($PfxPassword) { $params["Password"] = ConvertTo-SecureString $PfxPassword -AsPlainText -Force }
            if ($FriendlyName) { $params["FriendlyName"] = $FriendlyName }
            if ($PrivateKeyExportable) { $params["PrivateKeyExportable"] = ($PrivateKeyExportable -eq "true") }
            Import-ExchangeCertificate @params | Out-Null
            ConvertTo-BridgeCertificate (Read-BridgeCertificate)
        }

        "EnableServices" {
            $cert = Get-BridgeCertificate
            if ($null -eq $cert) { throw "The certificate $thumbprint does not exist on $certServer." }
            $current = @(Get-BridgeServices $cert)
            $missing = @(ConvertFrom-BridgeCsv $Services | Where-Object { $current -notcontains $_ })
            if ($missing.Count -gt 0) {
                # -Force: enabling SMTP asks whether to replace the default SMTP
                # certificate, which fails in a non-interactive session.
                Enable-ExchangeCertificate -Server $certServer -Thumbprint $thumbprint -Services ($missing -join ",") -Force
                $cert = Read-BridgeCertificate
            }
            ConvertTo-BridgeCertificate $cert
        }

        "Delete" {
            $cert = Get-BridgeCertificate
            if ($null -ne $cert) {
                $inUse = @(Get-BridgeServices $cert | Where-Object { $BoundServices -contains $_ })
                if ($inUse.Count -gt 0) {
                    throw "The certificate $thumbprint is still enabled for $($inUse -join ', ') on $certServer. Enable these services on another certificate first."
                }
                Remove-ExchangeCertificate -Server $certServer -Thumbprint $thumbprint @BridgeNoConfirm
            }
            "{}"
        }
    }
} catch {
    [Console]::Error.WriteLine("exchange_certificate $Action failed: $($_.Exception.Message)")
    exit 1
}
