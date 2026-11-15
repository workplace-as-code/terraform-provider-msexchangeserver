<#
.SYNOPSIS
    Checks that the parameter of every attribute exists on the cmdlet that
    writes it. Dot-sourced by test/run.ps1, once connected.
#>

# Get-RemoteParameterNames returns the parameter names of a cmdlet as the
# Exchange session shows them to the account (RBAC filters them), or $null
# when the session gives nothing usable. The endpoint runs in NoLanguage
# mode: the command is built as text, cmdlets and literals only.
function Get-RemoteParameterNames {
    param([Parameter(Mandatory)] $Session, [Parameter(Mandatory)] [string]$Cmdlet)
    $command = [scriptblock]::Create("Get-Command -Name '$Cmdlet' | Select-Object -ExpandProperty Parameters")
    $result = @(Invoke-Command -Session $Session -ScriptBlock $command -ErrorAction SilentlyContinue)
    $names = @()
    foreach ($item in $result) {
        if ($item -is [System.Collections.IDictionary]) {
            $names += @($item.Keys | ForEach-Object { [string]$_ })
        } elseif ($null -ne $item.PSObject.Properties["Key"]) {
            $names += [string]$item.Key
        } elseif ($null -ne $item.PSObject.Properties["Name"]) {
            $names += [string]$item.Name
        }
    }
    if ($names.Count -eq 0) { return $null }
    return ,$names
}

# Test-PreflightParameters reads "<cmdlet> <parameter>" lines and returns the
# number of errors. A parameter in no role of the organization does not exist
# in this Exchange version: an error. A parameter that exists but that the
# account cannot use is a warning. A cmdlet the account cannot run was already
# reported by the cmdlet check (-Commands lists what it can run).
function Test-PreflightParameters {
    param(
        [Parameter(Mandatory)] $Session,
        [Parameter(Mandatory)] [string]$Path,
        [string[]]$Commands = @()
    )
    $canReadRoles = $Commands -contains "Get-ManagementRoleEntry"
    $parameters = @{}
    $ok = 0
    $errors = 0
    foreach ($line in Get-Content -Path $Path) {
        if (-not $line -or $line.StartsWith("#")) { continue }
        $cmdlet, $parameter = $line -split " ", 2
        if ($Commands -notcontains $cmdlet) { continue }
        if (-not $parameters.ContainsKey($cmdlet)) {
            $parameters[$cmdlet] = Get-RemoteParameterNames -Session $Session -Cmdlet $cmdlet
        }
        $names = $parameters[$cmdlet]
        if ($null -eq $names) {
            Write-Host "  ! $line could not be checked: the session does not show the parameters of $cmdlet" -ForegroundColor Yellow
            continue
        }
        if ($names -contains $parameter) { $ok++; continue }
        $roles = @()
        if ($canReadRoles) {
            $query = [scriptblock]::Create("Get-ManagementRoleEntry -Identity '*\$cmdlet' -Parameters '$parameter' -ErrorAction SilentlyContinue")
            $roles = @(Invoke-Command -Session $Session -ScriptBlock $query -ErrorAction SilentlyContinue |
                ForEach-Object { [string]$_.Role } | Sort-Object -Unique)
        }
        if ($roles.Count -gt 0) {
            Write-Host "  ! $line is not available to the account (roles: $($roles -join ', '))" -ForegroundColor Yellow
        } elseif (-not $canReadRoles) {
            Write-Host "  ! $line is not available to the account (it cannot run Get-ManagementRoleEntry to say why)" -ForegroundColor Yellow
        } else {
            Write-Host "  x $line is in no role of the organization: this Exchange version has no such parameter" -ForegroundColor Red
            $errors++
        }
    }
    Write-Host "  ok: $ok"
    return $errors
}
