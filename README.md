# Durable Functions Go Fan-out/Fan-in Quickstart

This sample follows the [Go timer quickstart](https://github.com/Azure-Samples/functions-quickstart-go-azd-timer)
and deploys to Linux Flex Consumption with Azure Developer CLI (`azd`).
It uses Go worker and Durable middleware **v1.0.0** with **Durable Task Scheduler**.

`FanOutGreetings` schedules one `SayHello` activity for each city before awaiting
any results. Activities run independently; the orchestrator gathers their results
in input order. All durable state belongs to the Functions host and scheduler.
There is no `function.json` and no standalone Go task worker.

The worker and middleware have GA tags. The middleware currently depends on
`durabletask-go v1.0.0-beta3` and requires a bundle carrying DurableTask extension
**3.15.0 or later**. This sample uses the experimental bundle recommended by the
[published middleware documentation](https://github.com/Azure/azure-functions-golang-worker/tree/v1.0.0/middleware/durabletask).
These dependencies are pinned to published versions, not Git branches or local replacements.

## Prerequisites

- [Go 1.25 or later](https://go.dev/dl/)
- [Azure Functions Core Tools 4.12 or later](https://learn.microsoft.com/azure/azure-functions/functions-run-local)
- [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli)
- [Azure Developer CLI](https://learn.microsoft.com/azure/developer/azure-developer-cli/install-azd)
- Docker and [Azurite](https://learn.microsoft.com/azure/storage/common/storage-use-azurite) for local testing
- [PowerShell 7](https://learn.microsoft.com/powershell/scripting/install/installing-powershell) for the commands shown below
- An Azure subscription with permission to create resources and role assignments

## Run locally

```powershell
git clone https://github.com/Azure-Samples/durable-functions-quickstart-go-azd.git
cd durable-functions-quickstart-go-azd
Copy-Item local.settings.example.json local.settings.json
```

Start the scheduler emulator in one terminal. Custom hub names must be explicitly
enabled; the emulator otherwise exposes only `default`.

```powershell
docker run --rm -p 127.0.0.1:8080:8080 -p 127.0.0.1:8082:8082 -e DTS_TASK_HUB_NAMES=GoGreetings mcr.microsoft.com/dts/dts-emulator:latest
```

Start Azurite in a second terminal:

```powershell
azurite --silent --skipApiVersionCheck --location .azurite
```

Azurite provides Functions host storage; orchestration history uses the scheduler,
not Azurite. The unauthenticated scheduler connection is for local emulation only.

In the app directory:

```powershell
if ($IsWindows) {
    $env:languageWorkers__native__defaultExecutablePath = Join-Path (Get-Location) 'bin\app.exe'
}
func start
```

The process-only override works around Core Tools 4.12's Windows native worker
executable path. It is not needed for Linux deployment.

Then start a workflow and poll its status in another terminal:

```powershell
$start = Invoke-RestMethod -Method Post -Uri http://localhost:7071/api/greetings -ContentType application/json -Body '{"cities":["Tokyo","Seattle","London"]}'
Invoke-RestMethod -Uri $start.statusQueryGetUri
```

Repeat the status request until `runtimeStatus` is `Completed`. Expected output:

```json
["Hello, Tokyo!","Hello, Seattle!","Hello, London!"]
```

The starter returns HTTP 202 with the standard Durable management URLs.
`GET /api/greetings/{instanceId}` also exposes status. The emulator dashboard is
at `http://localhost:8082`. Stop the emulator and Azurite when finished.

## Deploy to Azure

```powershell
az login
azd auth login
azd env new go-greetings
azd env set VNET_ENABLED false
azd up
```

Choose a region supporting both Flex Consumption and
[Durable Task Scheduler](https://learn.microsoft.com/azure/durable-task/scheduler/durable-task-scheduler).
Infrastructure creates a Consumption scheduler, task hub, Linux function app,
host/deployment storage, monitoring, and user-assigned managed identity. The app
receives **Durable Task Data Contributor** scoped to its task hub. The scheduler
connection uses `DefaultAzure` authentication; storage uses managed identity and
disables shared keys. No secrets are required in committed configuration.

HTTP endpoints require a function key in Azure; local Core Tools does not require
one by default. Send the key in the `x-functions-key` header. Do not publish
management URLs: they can contain host system keys. The scheduler connection and
task hub dashboard URL are exported as `azd` outputs.

The optional VNet setting only privatizes host storage, not the scheduler.
The scheduler IP allowlist permits all IPv4 addresses so the sample can reach its
public endpoint; Microsoft Entra authentication and task hub RBAC remain required.
Restrict the allowlist or use private networking for production. Resource and telemetry
usage can incur charges; delete the sample environment with `azd down`.

## Development

```powershell
go test ./...
go vet ./...
```

Keep orchestrator code deterministic. Schedule I/O in activities, not inside
`FanOutGreetings`. Activities can execute more than once, so real side effects
must be idempotent. Unit tests cover input handling and activity output; they do
not substitute for a real host/scheduler run.
