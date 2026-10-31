<#
.SYNOPSIS
    Creates the least-privilege Exchange Server role group for the
    workplace-as-code/msexchangeserver Terraform provider and adds the
    provider's write account to it.

.DESCRIPTION
    The role group gets the management roles listed in $ProviderRoles, chosen
    by hand: the narrowest roles that contain the cmdlets the bridge scripts
    run. Retention Management is replaced by a custom child role that keeps
    only the cmdlets in $RetentionCmdlets: the built-in role can also set
    litigation holds and disable accounts. The script does not pick roles
    itself: it checks with Exchange that these roles still contain every
    cmdlet of the provider, and stops if one is missing.

    The read account needs no new role group: the built-in View-Only
    Organization Management role group is enough. The script checks that it
    contains every Get- cmdlet of the provider, and -ReadMember adds the read
    account to it.

    Without -Apply, the script only checks the roles and shows what it would do.

    Run it in the Exchange Management Shell as a member of Organization
    Management.

.PARAMETER Member
    Write account of the provider, as CONTOSO\svc-tf-exch-write.

.PARAMETER ReadMember
    Read account of the provider, as CONTOSO\svc-tf-exch-read. With -Apply,
    it is added to View-Only Organization Management.

.PARAMETER RoleGroupName
    Name of the role group to create.

.PARAMETER Apply
    Create the role group, or add the missing roles to it, and add the
    accounts. Without it, the script only shows what it would do.

.PARAMETER RemoveExtraRoles
    With -Apply, also remove from an existing role group the roles that are
    not in $ProviderRoles nor the custom retention role. Without it, the
    script only lists them.

.EXAMPLE
    .\new_role_group.ps1

    Checks the roles and shows what the script would change.

.EXAMPLE
    .\new_role_group.ps1 -Member CONTOSO\svc-tf-exch-write -ReadMember CONTOSO\svc-tf-exch-read -Apply
#>
[CmdletBinding()]
param(
    [string]$Member = "",
    [string]$ReadMember = "",
    [string]$RoleGroupName = "terraform-msexchangeserver",
    [switch]$Apply,
    [switch]$RemoveExtraRoles
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# The built-in management roles of the role group. Change this list only in a
# pull request, after checking what the new role allows: the docs list the
# same roles (a unit test compares them).
$ProviderRoles = @(
    "E-Mail Address Policies",
    "Organization Configuration",
    "Organization Transport Settings",
    "Recipient Policies",
    "Remote and Accepted Domains",
    "Transport Rules"
)

# The custom role that replaces Retention Management: a child of it that keeps
# only these cmdlets. The built-in role also grants Set-Mailbox for litigation
# holds and AccountDisabled, and New-Mailbox, which the provider never uses.
# Set-ADServerSettings comes with it: the provider runs it once per session
# when the domain_controller setting is used.
$RetentionParent = "Retention Management"
$RetentionRole = "$RoleGroupName Retention"
$RetentionCmdlets = @(
    "Get-Mailbox",
    "Get-RetentionPolicy",
    "Get-RetentionPolicyTag",
    "New-RetentionPolicy",
    "New-RetentionPolicyTag",
    "Remove-RetentionPolicy",
    "Remove-RetentionPolicyTag",
    "Set-ADServerSettings",
    "Set-RetentionPolicy",
    "Set-RetentionPolicyTag"
)

# The built-in role group of the read account.
$ReadRoleGroup = "View-Only Organization Management"

# Every Exchange cmdlet run by the provider's bridge scripts (bridge/scripts).
# A unit test (tools/tools_test.go) fails when this list drifts from the scripts.
$ProviderCmdlets = @(
    "Disable-TransportRule",
    "Enable-TransportRule",
    "Get-AcceptedDomain",
    "Get-EmailAddressPolicy",
    "Get-Mailbox",
    "Get-MobileDeviceMailboxPolicy",
    "Get-OrganizationConfig",
    "Get-OwaMailboxPolicy",
    "Get-RemoteDomain",
    "Get-RetentionPolicy",
    "Get-RetentionPolicyTag",
    "Get-TransportConfig",
    "Get-TransportRule",
    "New-AcceptedDomain",
    "New-EmailAddressPolicy",
    "New-MobileDeviceMailboxPolicy",
    "New-OwaMailboxPolicy",
    "New-RemoteDomain",
    "New-RetentionPolicy",
    "New-RetentionPolicyTag",
    "New-TransportRule",
    "Remove-AcceptedDomain",
    "Remove-EmailAddressPolicy",
    "Remove-MobileDeviceMailboxPolicy",
    "Remove-OwaMailboxPolicy",
    "Remove-RemoteDomain",
    "Remove-RetentionPolicy",
    "Remove-RetentionPolicyTag",
    "Remove-TransportRule",
    "Set-ADServerSettings",
    "Set-AcceptedDomain",
    "Set-EmailAddressPolicy",
    "Set-MobileDeviceMailboxPolicy",
    "Set-OrganizationConfig",
    "Set-OwaMailboxPolicy",
    "Set-RemoteDomain",
    "Set-RetentionPolicy",
    "Set-RetentionPolicyTag",
    "Set-TransportConfig",
    "Set-TransportRule"
)

# Find-UncoveredCmdlets returns the cmdlets that none of $Roles contains.
# $RolesByCmdlet maps each cmdlet to the names of the roles containing it.
function Find-UncoveredCmdlets {
    param([hashtable]$RolesByCmdlet, [string[]]$Roles)
    @($RolesByCmdlet.Keys | Where-Object {
        $cmdlet = $_
        -not @($RolesByCmdlet[$cmdlet] | Where-Object { $Roles -contains $_ })
    } | Sort-Object)
}

# Allows the test suite to load the functions without running anything.
if ($env:MSEXCHANGESERVER_ROLEGROUP_NO_RUN) { return }

if (-not (Get-Command Get-ManagementRole -ErrorAction SilentlyContinue)) {
    throw "Run this script in the Exchange Management Shell."
}

Write-Host "Checking that the roles contain the $($ProviderCmdlets.Count) cmdlets of the provider..."
$unknownRoles = @(@($ProviderRoles) + $RetentionParent | Where-Object { -not (Get-ManagementRole -Identity $_ -ErrorAction SilentlyContinue) })
if ($unknownRoles.Count -gt 0) {
    throw "These roles do not exist in this organization: $($unknownRoles -join ', ')"
}

$rolesByCmdlet = @{}
foreach ($cmdlet in $ProviderCmdlets) {
    $rolesByCmdlet[$cmdlet] = @(Get-ManagementRoleEntry "*\$cmdlet" -ErrorAction SilentlyContinue |
        ForEach-Object { [string]$_.Role } | Sort-Object -Unique)
}

# The custom retention role contains the cmdlets of $RetentionCmdlets that its
# parent contains, whether it exists yet or not.
$parentCmdlets = @(Get-ManagementRoleEntry "$RetentionParent\*" | ForEach-Object { [string]$_.Name })
$missingFromParent = @($RetentionCmdlets | Where-Object { $parentCmdlets -notcontains $_ })
if ($missingFromParent.Count -gt 0) {
    throw "$RetentionParent no longer contains: $($missingFromParent -join ', '). Please open an issue on the provider, with this output."
}
foreach ($cmdlet in $RetentionCmdlets) {
    if ($rolesByCmdlet.ContainsKey($cmdlet)) { $rolesByCmdlet[$cmdlet] = @($rolesByCmdlet[$cmdlet]) + $RetentionRole }
}
$groupRoles = @($ProviderRoles) + $RetentionRole
$uncovered = @(Find-UncoveredCmdlets -RolesByCmdlet $rolesByCmdlet -Roles $groupRoles)
if ($uncovered.Count -gt 0) {
    $details = $uncovered | ForEach-Object { "  - $_ (in: $(@($rolesByCmdlet[$_]) -join ', '))" }
    throw ("The roles of the provider no longer contain these cmdlets:`n$($details -join "`n")`n" +
        "This Exchange version has moved them: please open an issue on the provider, with this output.")
}

# The read account runs the Get- cmdlets, and Set-ADServerSettings with domain_controller.
$readRoles = @(Get-ManagementRoleAssignment -RoleAssignee $ReadRoleGroup -Delegating:$false |
    ForEach-Object { [string]$_.Role } | Sort-Object -Unique)
$readCmdlets = @{}
foreach ($cmdlet in $ProviderCmdlets | Where-Object { $_ -like "Get-*" -or $_ -eq "Set-ADServerSettings" }) {
    $readCmdlets[$cmdlet] = $rolesByCmdlet[$cmdlet]
}
$readUncovered = @(Find-UncoveredCmdlets -RolesByCmdlet $readCmdlets -Roles $readRoles)
if ($readUncovered.Count -gt 0) {
    Write-Warning "'$ReadRoleGroup' does not contain: $($readUncovered -join ', '). The read account cannot run them."
}

Write-Host ""
Write-Host "Management roles of '$RoleGroupName':"
$ProviderRoles | ForEach-Object { Write-Host "  - $_" }
Write-Host "  - $RetentionRole (custom role from ${RetentionParent}: $($RetentionCmdlets -join ', '))"

# The custom retention role: entries to add or remove to match $RetentionCmdlets.
$retentionExists = [bool](Get-ManagementRole -Identity $RetentionRole -ErrorAction SilentlyContinue)
$retentionEntries = @()
if ($retentionExists) { $retentionEntries = @(Get-ManagementRoleEntry "$RetentionRole\*" | ForEach-Object { [string]$_.Name }) }
$entriesToRemove = @($retentionEntries | Where-Object { $RetentionCmdlets -notcontains $_ })
$entriesToAdd = @($RetentionCmdlets | Where-Object { $retentionEntries -notcontains $_ })
if (-not $retentionExists) {
    Write-Host "The custom role '$RetentionRole' will be created."
} elseif ($entriesToRemove.Count + $entriesToAdd.Count -gt 0) {
    Write-Host "The custom role '$RetentionRole' will be updated: add $($entriesToAdd.Count) and remove $($entriesToRemove.Count) cmdlets."
}

# Roles of an existing role group that are not in the list.
$extra = @()
if (Get-RoleGroup -Identity $RoleGroupName -ErrorAction SilentlyContinue) {
    $current = @((Get-RoleGroup -Identity $RoleGroupName).Roles | ForEach-Object { (Get-ManagementRole -Identity $_).Name })
    $extra = @($current | Where-Object { $groupRoles -notcontains $_ } | Sort-Object)
}
if ($extra.Count -gt 0 -and -not ($Apply -and $RemoveExtraRoles)) {
    Write-Host ""
    Write-Warning ("'$RoleGroupName' has roles the provider does not need: $($extra -join ', '). " +
        "Remove them with -Apply -RemoveExtraRoles.")
}

if (-not $Apply) {
    Write-Host ""
    Write-Host "Nothing changed. Run again with -Member and -Apply to create or update '$RoleGroupName'."
    return
}
if (-not $Member) {
    throw "-Apply needs -Member."
}

# The custom role first, so that the role group never misses its cmdlets. A
# child role starts with every entry of its parent: the unneeded ones are removed.
if (-not $retentionExists) {
    New-ManagementRole -Name $RetentionRole -Parent $RetentionParent -Description "Retention policy cmdlets for the workplace-as-code/msexchangeserver Terraform provider" | Out-Null
    Write-Host "Created the custom role '$RetentionRole'."
    $entriesToRemove = @(Get-ManagementRoleEntry "$RetentionRole\*" | ForEach-Object { [string]$_.Name } | Where-Object { $RetentionCmdlets -notcontains $_ })
    $entriesToAdd = @()
}
foreach ($cmdlet in $entriesToRemove) {
    Remove-ManagementRoleEntry -Identity "$RetentionRole\$cmdlet" -Confirm:$false
}
foreach ($cmdlet in $entriesToAdd) {
    Add-ManagementRoleEntry -Identity "$RetentionRole\$cmdlet" | Out-Null
}
if ($entriesToRemove.Count + $entriesToAdd.Count -gt 0) {
    Write-Host "'$RetentionRole' now contains: $($RetentionCmdlets -join ', ')."
}

if (Get-RoleGroup -Identity $RoleGroupName -ErrorAction SilentlyContinue) {
    $current = @((Get-RoleGroup -Identity $RoleGroupName).Roles | ForEach-Object { (Get-ManagementRole -Identity $_).Name })
    foreach ($role in $groupRoles | Where-Object { $current -notcontains $_ }) {
        New-ManagementRoleAssignment -SecurityGroup $RoleGroupName -Role $role | Out-Null
        Write-Host "Added role '$role' to '$RoleGroupName'."
    }
    if ($RemoveExtraRoles) {
        foreach ($role in $extra) {
            Get-ManagementRoleAssignment -RoleAssignee $RoleGroupName -Role $role |
                Where-Object { $_.RoleAssigneeName -eq $RoleGroupName } |
                Remove-ManagementRoleAssignment -Confirm:$false
            Write-Host "Removed role '$role' from '$RoleGroupName'."
        }
    }
} else {
    New-RoleGroup -Name $RoleGroupName -Roles $groupRoles -Description "Least-privilege roles for the workplace-as-code/msexchangeserver Terraform provider" | Out-Null
    Write-Host "Created role group '$RoleGroupName'."
}

# Add-RoleGroupMember fails when the account is already a member.
function Add-MissingMember([string]$Group, [string]$Account) {
    $user = Get-User -Identity $Account
    $members = @(Get-RoleGroupMember -Identity $Group | ForEach-Object { [string]$_.Guid })
    if ($members -notcontains [string]$user.Guid) {
        Add-RoleGroupMember -Identity $Group -Member $user.Identity -BypassSecurityGroupManagerCheck
        Write-Host "Added $Account to '$Group'."
    }
}
Add-MissingMember $RoleGroupName $Member
if ($ReadMember) { Add-MissingMember $ReadRoleGroup $ReadMember }
Write-Host ("Done. Open a new session for the accounts to get their roles. Remove the write account from " +
    "Organization Management, or any other role group that grants more than the provider needs.")
