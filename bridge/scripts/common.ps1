# common.ps1 — Helpers shared by the bridge scripts.
# Dot-sourced after auth.ps1.

# ConvertTo-BridgeSplat builds the cmdlet parameters from the script parameters
# that the Go client actually sent. Attributes absent from the Terraform
# configuration are not sent, so Exchange keeps its current value for them.
#   -List  base64 JSON lists; "[]" clears the property
#   -Csv   comma-separated lists (older resources); "" clears the property
function ConvertTo-BridgeSplat {
    param(
        $Bound,
        [string[]]$Bool   = @(),
        [string[]]$Int    = @(),
        [string[]]$String = @(),
        [string[]]$List   = @(),
        [string[]]$Csv    = @()
    )
    $splat = @{}
    foreach ($k in $Bool)   { if ($Bound.ContainsKey($k)) { $splat[$k] = ([string]$Bound[$k] -eq "true") } }
    foreach ($k in $Int)    { if ($Bound.ContainsKey($k)) { $splat[$k] = [int64]$Bound[$k] } }
    foreach ($k in $String) { if ($Bound.ContainsKey($k)) { $splat[$k] = [string]$Bound[$k] } }
    foreach ($k in $List)   {
        if (-not $Bound.ContainsKey($k)) { continue }
        $items = @(ConvertFrom-BridgeList ([string]$Bound[$k]))
        $splat[$k] = if ($items.Count -gt 0) { [string[]]$items } else { $null }
    }
    foreach ($k in $Csv) {
        if (-not $Bound.ContainsKey($k)) { continue }
        $items = @(ConvertFrom-BridgeCsv ([string]$Bound[$k]))
        $splat[$k] = if ($items.Count -gt 0) { [string[]]$items } else { $null }
    }
    $splat
}

# ConvertFrom-BridgeCsv splits a comma-separated list ("" is an empty list).
function ConvertFrom-BridgeCsv {
    param([string]$Value)
    @($Value -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

# ConvertFrom-BridgeList decodes a list sent by the Go client: base64 JSON, or
# "[]" / "" for an empty list.
function ConvertFrom-BridgeList {
    param([string]$Value)
    if (-not $Value -or $Value -eq "[]") { return @() }
    $json = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
    # Assigned first: Windows PowerShell 5.1 sends a JSON array down the
    # pipeline as one object, so @($json | ConvertFrom-Json) would nest it.
    $items = ConvertFrom-Json -InputObject $json
    return @($items)
}

# ConvertFrom-BridgeJson decodes a base64 JSON value sent by the Go client.
function ConvertFrom-BridgeJson {
    param([string]$Value)
    if (-not $Value) { return $null }
    $json = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
    return ($json | ConvertFrom-Json)
}

# ConvertFrom-BridgeValues turns the base64 JSON object of parameter values sent
# by the generic resources into a splat hashtable. JSON arrays become string
# arrays; an empty array clears a multi-valued parameter.
function ConvertFrom-BridgeValues {
    param([string]$Value)
    $splat = @{}
    $object = ConvertFrom-BridgeJson $Value
    if ($null -eq $object) { return $splat }
    foreach ($p in $object.PSObject.Properties) {
        $v = $p.Value
        if ($v -is [array]) { $v = if ($v.Count -gt 0) { [string[]]$v } else { $null } }
        $splat[$p.Name] = $v
    }
    $splat
}

# ConvertTo-BridgeValue normalizes a property value for JSON: booleans and
# numbers stay typed, collections become string arrays, anything else (time
# spans, sizes, "Unlimited", ...) becomes a string.
function ConvertTo-BridgeValue {
    param($Value)
    if ($null -eq $Value) { return $null }
    if ($Value -is [bool] -or $Value -is [string]) { return $Value }
    if ($Value -is [int] -or $Value -is [long] -or $Value -is [uint32] -or $Value -is [uint64] -or
        $Value -is [int16] -or $Value -is [uint16] -or $Value -is [byte]) { return [long]$Value }
    if ($Value -is [System.Collections.IEnumerable]) {
        return ,([string[]]@($Value | Where-Object { $null -ne $_ } | ForEach-Object { $_.ToString() }))
    }
    return $Value.ToString()
}

# ConvertTo-BridgeStrings outputs the values of a multi-valued Exchange property
# as strings, skipping empty ones. Call it inside @() to get an array, also for
# an empty list: in a hashtable, Prop = if (...) { ... } else { @() } yields
# AutomationNull for an empty list, which Windows PowerShell 5.1 writes as {}
# in JSON instead of [].
function ConvertTo-BridgeStrings {
    param($Values)
    foreach ($value in $Values) {
        if ($null -ne $value -and "$value" -ne "") { "$value" }
    }
}

# Test-BridgeNotFound reports whether an error means the object does not exist.
function Test-BridgeNotFound {
    param([string]$Message)
    $Message -match "couldn't be found|could not be found|not found|ObjectNotFound|doesn't exist|does not exist"
}

# Get-BridgeProp reads a property that may be missing on some objects or module
# versions; under StrictMode a direct access to it would throw.
function Get-BridgeProp {
    param($Object, [string]$Name, $Default = $null)
    $p = $Object.PSObject.Properties[$Name]
    if ($null -ne $p) { $p.Value } else { $Default }
}

# Invoke-BridgeRetry runs a script block and retries it when Exchange
# reports a transient error (server-side error, throttling, timeout).
function Invoke-BridgeRetry {
    param([scriptblock]$Command, [int]$MaxAttempts = 4)
    for ($attempt = 1; ; $attempt++) {
        try {
            return & $Command
        } catch {
            $transient = $_.Exception.Message -match 'server side error|try again|throttl|\b429\b|\b503\b|timed out|temporarily unavailable'
            if (-not $transient -or $attempt -ge $MaxAttempts) { throw }
            Start-Sleep -Seconds (5 * $attempt)
        }
    }
}

# Wait-BridgeConsistent reads an object until a check passes. Exchange
# can return the previous value for a few seconds after a write.
function Wait-BridgeConsistent {
    param([scriptblock]$Read, [scriptblock]$Check, [int]$MaxAttempts = 6)
    for ($attempt = 1; ; $attempt++) {
        $object = & $Read
        if ((& $Check $object) -or $attempt -ge $MaxAttempts) { return $object }
        Start-Sleep -Seconds (2 * $attempt)
    }
}
