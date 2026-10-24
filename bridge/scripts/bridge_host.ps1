# bridge_host.ps1 — Persistent host for the PowerShell bridge scripts.
#
# The Go client starts a few long-lived PowerShell processes running this script
# and sends them requests, instead of starting one process per Terraform
# operation. The Exchange connection (auth.ps1) is then opened once per process
# and reused, which removes the remote session setup (New-PSSession, Import-PSSession) from
# every operation after the first.
#
# PROTOCOL (one request at a time, everything ASCII):
#   stdin  : one line per request = base64(UTF-8 JSON)
#            {"id": 1, "script": "transport_rule.ps1", "params": {"Action": "Get", ...}}
#   stdout : one line per response = "<nonce> " + base64(UTF-8 JSON)
#            {"id": 1, "exitCode": 0, "stdout": "...", "stderr": "..."}
#   On startup the host sends {"id": 0, "ready": true}.
#   Lines on stdout without the nonce prefix are noise (e.g. text written straight
#   to the console by a module) and are ignored by the client.
#   EOF on stdin = shutdown: the host disconnects from Exchange and exits.
#
# The nonce comes from EXCHANGE_BRIDGE_NONCE, a random value chosen by the client
# for each process, so script output can never be mistaken for a response.
# Secrets are never in requests: auth.ps1 reads them from the environment.

param(
    [string]$Server   = "",
    [string]$Username = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference    = "SilentlyContinue"

$nonce = $env:EXCHANGE_BRIDGE_NONCE
if (-not $nonce) {
    [Console]::Error.WriteLine("EXCHANGE_BRIDGE_NONCE is not set; bridge_host.ps1 must be started by the provider")
    exit 2
}

$utf8     = New-Object System.Text.UTF8Encoding($false)
$protoOut = [Console]::Out
$origErr  = [Console]::Error
$stdin    = [Console]::In

# Non-secret connection parameters, added to every script invocation.
$authParams = @{
    Server   = $Server
    Username = $Username
}

# ConvertFrom-BridgeRequest decodes one request line into its id, script name
# and parameters, all as strings.
function ConvertFrom-BridgeRequest([string]$Line) {
    $json = $utf8.GetString([Convert]::FromBase64String($Line))
    $request = @{ id = -1; script = ""; params = @{} }
    # Windows PowerShell 5.1 has no System.Text.Json, and its ConvertFrom-Json
    # keeps ISO 8601 strings as strings: parameters reach the scripts unchanged.
    $root = ConvertFrom-Json -InputObject $json
    $request.id     = [int]$root.id
    $request.script = [string]$root.script
    $params = $root.PSObject.Properties["params"]
    if ($null -ne $params -and $null -ne $params.Value) {
        foreach ($p in $params.Value.PSObject.Properties) { $request.params[$p.Name] = [string]$p.Value }
    }
    return $request
}

function Send-Response($Response) {
    $json = $Response | ConvertTo-Json -Compress -Depth 4
    $protoOut.WriteLine($nonce + " " + [Convert]::ToBase64String($utf8.GetBytes($json)))
    $protoOut.Flush()
}

Send-Response @{ id = 0; ready = $true }

while ($true) {
    $line = $stdin.ReadLine()
    if ($null -eq $line) { break }          # client closed stdin: shut down
    if (-not $line.Trim()) { continue }

    $id      = -1
    $code    = 0
    $outText = ""
    $errBuf  = New-Object System.IO.StringWriter
    $noise   = New-Object System.IO.StringWriter

    try {
        $req  = ConvertFrom-BridgeRequest $line.Trim()
        $id   = $req.id
        $name = $req.script

        # Only bare script names from this folder; never this host itself.
        if ($name -notmatch '^[a-z0-9_]+\.ps1$' -or $name -eq "bridge_host.ps1") {
            throw "invalid script name '$name'"
        }
        $path = Join-Path $PSScriptRoot $name
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "unknown bridge script '$name'" }

        $splat = @{} + $authParams
        foreach ($k in $req.params.Keys) { $splat[$k] = $req.params[$k] }

        # Capture what the script writes straight to the console streams, so
        # each response carries only its own messages.
        [Console]::SetError($errBuf)
        [Console]::SetOut($noise)
        $global:LASTEXITCODE = 0

        # The script runs in its own scope: its variables, StrictMode and
        # preferences do not leak into the next request. 'exit N' inside it
        # only ends the script and sets $LASTEXITCODE.
        $result = & $path @splat 2>&1 3>$null 4>$null 5>$null 6>$null
        $code = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }

        $lines = foreach ($r in $result) {
            if ($r -is [System.Management.Automation.ErrorRecord]) { $errBuf.WriteLine($r.ToString()) }
            else { "$r" }
        }
        $outText = @($lines) -join "`n"
    } catch {
        $code = 1
        $errBuf.WriteLine($_.Exception.Message)
    } finally {
        [Console]::SetError($origErr)
        [Console]::SetOut($protoOut)
    }

    # Console.Out noise would have corrupted stdout in one-shot mode; keep it
    # only as diagnostic context on failures.
    $stderrText = $errBuf.ToString()
    if ($code -ne 0 -and $noise.ToString().Trim()) { $stderrText += "`n" + $noise.ToString() }

    Send-Response @{ id = $id; exitCode = $code; stdout = $outText; stderr = $stderrText.Trim() }
}

# Graceful shutdown: release the Exchange session instead of letting it expire.
try {
    Get-PSSession | Where-Object { $_.ConfigurationName -eq "Microsoft.Exchange" } | Remove-PSSession
} catch { }
exit 0
