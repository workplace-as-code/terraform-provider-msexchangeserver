---
page_title: "Detect drift in CI"
subcategory: "Guides"
---

# Detect drift in CI

Exchange settings also change outside Terraform: in the Exchange admin center, in the Exchange Management Shell, or through another tool. Such a change is a **drift**: the organization no longer matches your configuration, and the next `terraform apply` would silently revert it.

A scheduled `terraform plan` finds drifts early. It reads every object and setting your configuration manages, compares them with the code, and fails when they differ. It never changes the organization.

## How it works

`terraform plan -detailed-exitcode` tells the result through its exit code:

| Exit code | Meaning | Job result |
|---|---|---|
| `0` | The organization matches the configuration | Success |
| `1` | The plan failed (Kerberos, permissions, network...) | Failure |
| `2` | The organization differs from the configuration | Failure: someone needs to look |

Run it on a schedule, on the branch you deploy from, with the same state backend and service account as your deployment pipeline. Your CI then notifies you of a failed run like any other failed job.

## The runner

The provider runs on a **Windows machine joined to the Active Directory domain** and opens a Kerberos session to an Exchange server (see the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/authentication)). Runners hosted by GitHub or GitLab cannot reach your Exchange servers: the drift job needs a **self-hosted runner** on such a machine, with Terraform installed.

The examples below run in PowerShell on that runner. A native command does not stop a PowerShell script when it fails, so each `terraform` call checks `$LASTEXITCODE` and passes it on: exit code `2` must fail the job.

## GitHub Actions

```yaml
# .github/workflows/drift.yml
name: Drift detection

on:
  schedule:
    - cron: "0 5 * * 1-5" # every working day at 05:00 UTC
  workflow_dispatch:

permissions:
  contents: read

jobs:
  drift:
    runs-on: [self-hosted, windows]
    # An environment restricted to the default branch keeps the service
    # account password away from pull requests.
    environment: production
    steps:
      - uses: actions/checkout@v7

      - name: Plan
        shell: powershell
        env:
          EXCHANGE_SERVER: exch01.contoso.local
          EXCHANGE_USERNAME: ${{ secrets.EXCHANGE_USERNAME }}
          EXCHANGE_PASSWORD: ${{ secrets.EXCHANGE_PASSWORD }}
          # Add the credentials of your state backend here.
        run: |
          terraform init -input=false
          if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
          terraform plan -detailed-exitcode -input=false -no-color
          exit $LASTEXITCODE
```

In production, pin the actions to a commit SHA instead of a tag.

## GitLab CI

With a GitLab Runner registered on the Windows machine (shell executor, PowerShell), add a job that only runs in scheduled pipelines, next to your `plan` and `apply` jobs:

```yaml
drift:
  stage: plan
  tags: [windows, exchange] # the tags of your self-hosted runner
  rules:
    - if: $CI_PIPELINE_SOURCE == "schedule"
  resource_group: exchange-organization # never at the same time as an apply
  script:
    - terraform init -input=false
    - if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    - terraform plan -detailed-exitcode -input=false
    - exit $LASTEXITCODE
```

`EXCHANGE_SERVER`, `EXCHANGE_USERNAME` and `EXCHANGE_PASSWORD` are CI/CD variables of the project (the password masked; protected if only protected branches run this job). With the GitLab-managed Terraform state, the `TF_HTTP_*` variables of your pipeline configure the backend.

Then create the schedule in **Build > Pipeline schedules**, for example with the interval `0 5 * * 1-5` on the default branch, and run it once with the play button to check it.

## Read the result

A drift appears in the plan as a change of the attribute that was modified outside Terraform:

```text
  # msexchangeserver_remote_domain.partner will be updated in-place
  ~ resource "msexchangeserver_remote_domain" "partner" {
      ~ auto_forward_enabled = true -> false
        id                   = "Contoso-Partner"
        name                 = "Contoso-Partner"
        # (12 unchanged attributes hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.
```

The value on the left is the one in Exchange today, the value on the right is the one in your configuration: someone allowed automatic forwarding to the partner domain, and the next apply would block it again.

An object deleted outside Terraform appears as `will be created`.

## Resolve a drift

Pick one of three answers:

1. **The configuration is right**: the change was a mistake. Run your deployment pipeline: `apply` sets the value back.
2. **The organization is right**: the change was intended. Update the configuration to the new value through a merge request, so that the code describes the organization again.
3. **This setting should not be managed by Terraform**: remove the attribute from the configuration. The provider only manages the attributes set in the configuration, so an attribute left out is never written and no longer appears in the plan: the admins can change it freely.

## Limits

- The plan only checks what the configuration manages. A new object created in the admin center is not a drift: run [`terraform query`](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/import_existing) from time to time to find objects that are not managed yet.
- The plan reads every managed object: with many objects, expect a few minutes per run, and schedule it outside your deployment hours. `EXCHANGE_MAX_CONCURRENCY` limits the load on the Exchange server.
- On an Edge Transport server, the runner is the Edge server itself, or a machine that reaches it through WinRM: see the Edge section of the [Authentication guide](https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs/guides/authentication).
