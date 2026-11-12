<#
.SYNOPSIS
    Creates the least-privilege Exchange Server role group for the
    workplace-as-code/msexchangeserver Terraform provider and adds the
    provider's write account to it.

.DESCRIPTION
    The role group gets the management roles listed in $ProviderRoles, chosen
    by hand: the narrowest roles that contain the cmdlets the bridge scripts
    run. When the narrowest built-in role grants much more than the provider
    needs, the group gets a custom child role instead, which keeps only the
    cmdlets listed in $CustomRoles. The script does not pick roles
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

.PARAMETER CustomRolePrefix
    Prefix of the custom role names, followed by the name of each role in
    $CustomRoles (for example CR-EXCH-TF- gives CR-EXCH-TF-Certificates).
    Without it, the custom roles are named "<RoleGroupName> <name>".

.PARAMETER CustomRoleNames
    Name of each custom role, by its name in $CustomRoles, for naming
    conventions: @{ Certificates = "CR-EXCH-TF-CERT"; Databases = "CR-EXCH-TF-DB" }.
    It takes precedence over -CustomRolePrefix; the roles it does not list use
    the prefix.

.PARAMETER Apply
    Create the role group, or add the missing roles to it, and add the
    accounts. Without it, the script only shows what it would do.

.PARAMETER RemoveExtraRoles
    With -Apply, also remove from an existing role group the roles that are
    not in $ProviderRoles nor $CustomRoles. Without it, the script only lists
    them. A custom role that this script created and that nothing else is
    assigned is then deleted: after a rename, the old roles go away.

.EXAMPLE
    .\new_role_group.ps1

    Checks the roles and shows what the script would change.

.EXAMPLE
    .\new_role_group.ps1 -Member CONTOSO\svc-tf-exch-write -ReadMember CONTOSO\svc-tf-exch-read -Apply

.EXAMPLE
    .\new_role_group.ps1 -RoleGroupName RG-EXCH-TERRAFORM -CustomRolePrefix CR-EXCH-TF- -CustomRoleNames @{ Certificates = "CR-EXCH-TF-CERT" }

    Checks the roles with the naming convention of the organization.
#>
[CmdletBinding()]
param(
    [string]$Member = "",
    [string]$ReadMember = "",
    [string]$RoleGroupName = "terraform-msexchangeserver",
    [string]$CustomRolePrefix = "",
    [hashtable]$CustomRoleNames = @{},
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
    "POP3 And IMAP4 Protocols",
    "Receive Connectors",
    "Recipient Policies",
    "Remote and Accepted Domains",
    "Send Connectors",
    "Transport Rules"
)

# The custom roles: each one is a child of the Parent built-in role that keeps
# only Cmdlets, and is named after the role group followed by Name. A unit test
# checks that each list holds every cmdlet the bridge runs on its nouns. What
# each built-in role grants beyond the provider's needs:
# - Retention Management: Set-Mailbox for litigation holds and AccountDisabled,
#   and New-Mailbox. Set-ADServerSettings comes with the custom role: the
#   provider runs it once per session when domain_controller is set.
# - View-Only Recipients: Export-MailboxDiagnosticLogs and Set-UnifiedAuditSetting.
# - Exchange Virtual Directories: creates and removes virtual directories,
#   the PowerShell one included, and changes Set-CASMailbox.
# - Exchange Servers: DAG membership, server component states (maintenance
#   mode), database copies and every server setting. It is also the parent of
#   Transport Services: Transport Hygiene, the other role with
#   Set-TransportService, changes every anti-spam setting.
# - Databases: mounts, dismounts, moves and removes databases and mailboxes.
# - Exchange Server Certificates: Export-ExchangeCertificate exports private keys.
# - Audit Logs: Set-Mailbox, mailbox audit bypass and the audit log searches.
$CustomRoles = @(
    @{ Name = "Retention"; Parent = "Retention Management"; Cmdlets = @(
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
    ) },
    @{ Name = "Remote Mailboxes"; Parent = "View-Only Recipients"; Cmdlets = @(
        "Get-RemoteMailbox"
    ) },
    @{ Name = "Virtual Directories"; Parent = "Exchange Virtual Directories"; Cmdlets = @(
        "Get-ActiveSyncVirtualDirectory",
        "Get-AutodiscoverVirtualDirectory",
        "Get-EcpVirtualDirectory",
        "Get-MapiVirtualDirectory",
        "Get-OabVirtualDirectory",
        "Get-OwaVirtualDirectory",
        "Get-WebServicesVirtualDirectory",
        "Set-ActiveSyncVirtualDirectory",
        "Set-AutodiscoverVirtualDirectory",
        "Set-EcpVirtualDirectory",
        "Set-MapiVirtualDirectory",
        "Set-OabVirtualDirectory",
        "Set-OwaVirtualDirectory",
        "Set-WebServicesVirtualDirectory"
    ) },
    # Set-MapiVirtualDirectory -IISAuthenticationMethods is in Exchange Servers,
    # not in Exchange Virtual Directories: RBAC adds up both roles' parameters.
    # Exchange Servers has no Get-MapiVirtualDirectory: Virtual Directories
    # keeps it. Outlook Anywhere comes from Exchange Servers too: in Exchange
    # Virtual Directories, Get-OutlookAnywhere has no -Identity nor -Server.
    @{ Name = "Client Access"; Parent = "Exchange Servers"; Cmdlets = @(
        "Get-ClientAccessService",
        "Get-OutlookAnywhere",
        "Set-ClientAccessService",
        "Set-MapiVirtualDirectory",
        "Set-OutlookAnywhere"
    ) },
    @{ Name = "Transport Services"; Parent = "Exchange Servers"; Cmdlets = @(
        "Get-ExchangeServer",
        "Get-FrontendTransportService",
        "Get-TransportService",
        "Set-FrontendTransportService",
        "Set-TransportService"
    ) },
    @{ Name = "Databases"; Parent = "Databases"; Cmdlets = @(
        "Get-MailboxDatabase",
        "Set-MailboxDatabase"
    ) },
    @{ Name = "Audit Log"; Parent = "Audit Logs"; Cmdlets = @(
        "Get-AdminAuditLogConfig",
        "Set-AdminAuditLogConfig"
    ) },
    @{ Name = "Certificates"; Parent = "Exchange Server Certificates"; Cmdlets = @(
        "Enable-ExchangeCertificate",
        "Get-ExchangeCertificate",
        "Import-ExchangeCertificate",
        "Remove-ExchangeCertificate"
    ) }
)
# The name of each custom role in the organization: -CustomRoleNames, then
# -CustomRolePrefix, then "<RoleGroupName> <name>".
$unknownNames = @($CustomRoleNames.Keys | Where-Object { @($CustomRoles | ForEach-Object { $_.Name }) -notcontains $_ })
if ($unknownNames.Count -gt 0) {
    throw "-CustomRoleNames: unknown custom roles $($unknownNames -join ', '). Valid names: $(($CustomRoles | ForEach-Object { $_.Name }) -join ', ')."
}
foreach ($custom in $CustomRoles) {
    $custom.Role = if ($CustomRoleNames.ContainsKey($custom.Name)) { [string]$CustomRoleNames[$custom.Name] }
                   elseif ($CustomRolePrefix) { "$CustomRolePrefix$($custom.Name)" }
                   else { "$RoleGroupName $($custom.Name)" }
}
$duplicates = @($CustomRoles | Group-Object { $_.Role } | Where-Object { $_.Count -gt 1 } | ForEach-Object { $_.Name })
if ($duplicates.Count -gt 0) { throw "Several custom roles would have the same name: $($duplicates -join ', ')." }

# The built-in role group of the read account.
$ReadRoleGroup = "View-Only Organization Management"

# Every Exchange cmdlet run by the provider's bridge scripts (bridge/scripts).
# A unit test (tools/tools_test.go) fails when this list drifts from the scripts.
$ProviderCmdlets = @(
    "Disable-TransportRule",
    "Enable-ExchangeCertificate",
    "Enable-TransportRule",
    "Get-AcceptedDomain",
    "Get-ActiveSyncVirtualDirectory",
    "Get-AdminAuditLogConfig",
    "Get-ClientAccessService",
    "Get-EcpVirtualDirectory",
    "Get-EmailAddressPolicy",
    "Get-ExchangeCertificate",
    "Get-ExchangeServer",
    "Get-FrontendTransportService",
    "Get-ImapSettings",
    "Get-Mailbox",
    "Get-MailboxDatabase",
    "Get-MapiVirtualDirectory",
    "Get-MobileDeviceMailboxPolicy",
    "Get-OabVirtualDirectory",
    "Get-OrganizationConfig",
    "Get-OutlookAnywhere",
    "Get-OwaMailboxPolicy",
    "Get-OwaVirtualDirectory",
    "Get-PopSettings",
    "Get-ReceiveConnector",
    "Get-RemoteDomain",
    "Get-RemoteMailbox",
    "Get-RetentionPolicy",
    "Get-RetentionPolicyTag",
    "Get-SendConnector",
    "Get-TransportConfig",
    "Get-TransportRule",
    "Get-TransportService",
    "Get-WebServicesVirtualDirectory",
    "Import-ExchangeCertificate",
    "New-AcceptedDomain",
    "New-EmailAddressPolicy",
    "New-MobileDeviceMailboxPolicy",
    "New-OwaMailboxPolicy",
    "New-ReceiveConnector",
    "New-RemoteDomain",
    "New-RetentionPolicy",
    "New-RetentionPolicyTag",
    "New-SendConnector",
    "New-TransportRule",
    "Remove-AcceptedDomain",
    "Remove-EmailAddressPolicy",
    "Remove-ExchangeCertificate",
    "Remove-MobileDeviceMailboxPolicy",
    "Remove-OwaMailboxPolicy",
    "Remove-ReceiveConnector",
    "Remove-RemoteDomain",
    "Remove-RetentionPolicy",
    "Remove-RetentionPolicyTag",
    "Remove-SendConnector",
    "Remove-TransportRule",
    "Set-ADServerSettings",
    "Set-AcceptedDomain",
    "Set-ActiveSyncVirtualDirectory",
    "Set-AdminAuditLogConfig",
    "Set-ClientAccessService",
    "Set-EcpVirtualDirectory",
    "Set-EmailAddressPolicy",
    "Set-FrontendTransportService",
    "Set-ImapSettings",
    "Set-MailboxDatabase",
    "Set-MapiVirtualDirectory",
    "Set-MobileDeviceMailboxPolicy",
    "Set-OabVirtualDirectory",
    "Set-OrganizationConfig",
    "Set-OutlookAnywhere",
    "Set-OwaMailboxPolicy",
    "Set-OwaVirtualDirectory",
    "Set-PopSettings",
    "Set-ReceiveConnector",
    "Set-RemoteDomain",
    "Set-RetentionPolicy",
    "Set-RetentionPolicyTag",
    "Set-SendConnector",
    "Set-TransportConfig",
    "Set-TransportRule",
    "Set-TransportService",
    "Set-WebServicesVirtualDirectory"
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
$unknownRoles = @(@($ProviderRoles) + @($CustomRoles | ForEach-Object { $_.Parent }) | Where-Object { -not (Get-ManagementRole -Identity $_ -ErrorAction SilentlyContinue) })
if ($unknownRoles.Count -gt 0) {
    throw "These roles do not exist in this organization: $($unknownRoles -join ', ')"
}

$rolesByCmdlet = @{}
foreach ($cmdlet in $ProviderCmdlets) {
    $rolesByCmdlet[$cmdlet] = @(Get-ManagementRoleEntry "*\$cmdlet" -ErrorAction SilentlyContinue |
        ForEach-Object { [string]$_.Role } | Sort-Object -Unique)
}

# Each custom role contains the cmdlets of its list that its parent contains,
# whether it exists yet or not.
foreach ($custom in $CustomRoles) {
    $parentCmdlets = @(Get-ManagementRoleEntry "$($custom.Parent)\*" | ForEach-Object { [string]$_.Name })
    $missingFromParent = @($custom.Cmdlets | Where-Object { $parentCmdlets -notcontains $_ })
    if ($missingFromParent.Count -gt 0) {
        throw "$($custom.Parent) no longer contains: $($missingFromParent -join ', '). Please open an issue on the provider, with this output."
    }
    foreach ($cmdlet in $custom.Cmdlets) {
        if ($rolesByCmdlet.ContainsKey($cmdlet)) { $rolesByCmdlet[$cmdlet] = @($rolesByCmdlet[$cmdlet]) + $custom.Role }
    }
}
$groupRoles = @($ProviderRoles) + @($CustomRoles | ForEach-Object { $_.Role })
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
$CustomRoles | ForEach-Object { Write-Host "  - $($_.Role) (custom role from $($_.Parent): $($_.Cmdlets -join ', '))" }

# Each custom role: entries to add or remove to match its list.
foreach ($custom in $CustomRoles) {
    $custom.Exists = [bool](Get-ManagementRole -Identity $custom.Role -ErrorAction SilentlyContinue)
    $entries = @()
    if ($custom.Exists) { $entries = @(Get-ManagementRoleEntry "$($custom.Role)\*" | ForEach-Object { [string]$_.Name }) }
    $custom.ToRemove = @($entries | Where-Object { $custom.Cmdlets -notcontains $_ })
    $custom.ToAdd = @($custom.Cmdlets | Where-Object { $entries -notcontains $_ })
    if (-not $custom.Exists) {
        Write-Host "The custom role '$($custom.Role)' will be created."
    } elseif ($custom.ToRemove.Count + $custom.ToAdd.Count -gt 0) {
        Write-Host "The custom role '$($custom.Role)' will be updated: add $($custom.ToAdd.Count) and remove $($custom.ToRemove.Count) cmdlets."
    }
}

# The description of the custom roles: it marks the roles this script created,
# which -RemoveExtraRoles may delete.
$CustomRoleDescription = "for the workplace-as-code/msexchangeserver Terraform provider"

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

# The custom roles first, so that the role group never misses their cmdlets. A
# child role starts with every entry of its parent: the unneeded ones are removed.
foreach ($custom in $CustomRoles) {
    if (-not $custom.Exists) {
        New-ManagementRole -Name $custom.Role -Parent $custom.Parent -Description "Cmdlets of $($custom.Parent) $CustomRoleDescription" | Out-Null
        Write-Host "Created the custom role '$($custom.Role)'."
        $custom.ToRemove = @(Get-ManagementRoleEntry "$($custom.Role)\*" | ForEach-Object { [string]$_.Name } | Where-Object { $custom.Cmdlets -notcontains $_ })
        $custom.ToAdd = @()
    }
    foreach ($cmdlet in $custom.ToRemove) {
        Remove-ManagementRoleEntry -Identity "$($custom.Role)\$cmdlet" -Confirm:$false
    }
    foreach ($cmdlet in $custom.ToAdd) {
        Add-ManagementRoleEntry -Identity "$($custom.Role)\$cmdlet" | Out-Null
    }
    if ($custom.ToRemove.Count + $custom.ToAdd.Count -gt 0) {
        Write-Host "'$($custom.Role)' now contains: $($custom.Cmdlets -join ', ')."
    }
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
            # A custom role of this script that nothing else uses, such as the
            # old name of a renamed role.
            $old = Get-ManagementRole -Identity $role
            if (-not $old.IsRootRole -and "$($old.Description)" -like "*$CustomRoleDescription" -and
                @(Get-ManagementRoleAssignment -Role $role -Delegating:$false).Count -eq 0) {
                Remove-ManagementRole -Identity $role -Confirm:$false
                Write-Host "Deleted the custom role '$role'."
            }
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
