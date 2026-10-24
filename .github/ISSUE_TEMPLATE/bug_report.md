---
name: Bug report
about: A resource or data source does not behave as documented
labels: bug
---

## Versions

- Provider version:
- Terraform version (`terraform version`):
- Windows PowerShell version (`$PSVersionTable.PSVersion`):
- Exchange Server version and cumulative update (`Get-ExchangeServer | Format-List Name, AdminDisplayVersion`):
- Windows version of the machine running Terraform:

## Affected resources

<!-- e.g. msexchangeserver_... -->

## Configuration

```hcl
# Minimal configuration that reproduces the problem. Remove server names, account names and secrets.
```

## Expected behavior

## Actual behavior

<!-- Paste the error. For more detail, run with TF_LOG=DEBUG and attach the relevant part of the log, with secrets removed. -->

## Steps to reproduce

1. `terraform apply`
