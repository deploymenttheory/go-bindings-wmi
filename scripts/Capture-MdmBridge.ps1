<#
.SYNOPSIS
    Captures a CIM schema snapshot from a namespace that requires the SYSTEM
    account — chiefly the MDM bridge (root\cimv2\mdm\dmmap), whose WMI
    provider denies access to anything below SYSTEM (elevation is not
    enough).

.DESCRIPTION
    The go-bindings-wmi capture tool talks to the live CIM repository as the
    calling account. The MDM Bridge WMI provider only answers the SYSTEM
    account, so this script:

      1. builds cmd/capture into a self-contained exe (as the invoking admin),
      2. runs that exe as SYSTEM via a transient one-shot scheduled task,
      3. surfaces the exe's output and exit code,
      4. deletes the task and the temporary exe.

    The captured snapshot lands in metadata/cim/ like any other. Follow with
    `go run ./cmd/generate` to produce bindings/cim/dmmap.

    Must be run from an ELEVATED PowerShell (creating a SYSTEM task needs it),
    with Go on PATH.

.PARAMETER Namespace
    The CIM namespace to capture. Defaults to the MDM bridge.

.PARAMETER OSBuild
    Overrides the provenance OS build (default: read from the live OS).

.PARAMETER Captured
    Overrides the provenance date, YYYY-MM-DD (default: today, UTC).

.EXAMPLE
    # From an elevated PowerShell, at the repo root:
    .\scripts\Capture-MdmBridge.ps1
    go run ./cmd/generate
#>
[CmdletBinding()]
param(
    [string]$Namespace = 'root\cimv2\mdm\dmmap',
    [string]$OSBuild = '',
    [string]$Captured = ''
)

$ErrorActionPreference = 'Stop'

function Fail($message) {
    Write-Error $message
    exit 1
}

# --- preconditions -------------------------------------------------------
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Fail 'This script must run from an ELEVATED PowerShell (it creates a SYSTEM scheduled task).'
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Fail 'Go was not found on PATH.'
}

# Repo root is the parent of this script's directory.
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $metadataDir = Join-Path $repoRoot 'metadata\cim'
    New-Item -ItemType Directory -Force $metadataDir | Out-Null

    # --- build the capture exe (as the invoking admin) -------------------
    # A prebuilt static exe is far more robust than `go run` under SYSTEM,
    # whose PATH/GOCACHE differ. The win32 bindings lazy-load their DLLs from
    # System32, which SYSTEM can reach.
    $exe = Join-Path $env:TEMP ('capture-{0}.exe' -f ([guid]::NewGuid().ToString('N')))
    $log = Join-Path $env:TEMP ('capture-{0}.log' -f ([guid]::NewGuid().ToString('N')))
    Write-Host "Building capture tool -> $exe"
    & go build -o $exe ./cmd/capture
    if ($LASTEXITCODE -ne 0) { Fail 'go build ./cmd/capture failed.' }

    # --- assemble the capture command ------------------------------------
    $captureArgs = @('-namespace', $Namespace, '-out', $metadataDir)
    if ($OSBuild)  { $captureArgs += @('-osbuild', $OSBuild) }
    if ($Captured) { $captureArgs += @('-captured', $Captured) }
    $quotedArgs = ($captureArgs | ForEach-Object { '"{0}"' -f $_ }) -join ' '
    # Redirect output to a log; scheduled tasks have no console.
    $command = 'cmd.exe /c ""{0}" {1} > "{2}" 2>&1"' -f $exe, $quotedArgs, $log

    # --- run it as SYSTEM via a transient scheduled task -----------------
    $taskName = 'go-bindings-wmi-capture-{0}' -f ([guid]::NewGuid().ToString('N'))
    Write-Host "Running capture as SYSTEM (task $taskName)"
    & schtasks.exe /Create /TN $taskName /TR $command /SC ONCE /ST 00:00 /RU SYSTEM /RL HIGHEST /F | Out-Null
    if ($LASTEXITCODE -ne 0) { Fail 'Failed to create the SYSTEM scheduled task.' }
    try {
        & schtasks.exe /Run /TN $taskName | Out-Null
        if ($LASTEXITCODE -ne 0) { Fail 'Failed to start the SYSTEM scheduled task.' }

        # Poll until the task stops (Status transitions away from Running).
        $deadline = (Get-Date).AddMinutes(5)
        do {
            Start-Sleep -Milliseconds 500
            $status = (& schtasks.exe /Query /TN $taskName /FO LIST |
                Select-String -Pattern '^\s*Status:\s*(.+)$').Matches.Groups[1].Value.Trim()
        } while ($status -eq 'Running' -and (Get-Date) -lt $deadline)

        $result = (& schtasks.exe /Query /TN $taskName /FO LIST /V |
            Select-String -Pattern '^\s*Last Result:\s*(.+)$').Matches.Groups[1].Value.Trim()
    }
    finally {
        & schtasks.exe /Delete /TN $taskName /F | Out-Null
    }

    # --- surface the capture output --------------------------------------
    if (Test-Path $log) {
        Write-Host '--- capture output ---'
        Get-Content $log | Write-Host
        Remove-Item $log -Force
    }
    Remove-Item $exe -Force -ErrorAction SilentlyContinue

    if ($result -ne '0') {
        Fail "Capture (as SYSTEM) exited with code $result. See the output above — access-denied here usually means the MDM bridge is unavailable on this host (not enrolled)."
    }

    $leaf = ($Namespace -split '\\')[-1].ToLower()
    $snapshot = Join-Path $metadataDir (($Namespace -replace '\\', '.') + '.json')
    Write-Host ''
    Write-Host "Captured -> $snapshot"
    Write-Host "Next: go run ./cmd/generate   (produces bindings/cim/$leaf)"
}
finally {
    Pop-Location
}
