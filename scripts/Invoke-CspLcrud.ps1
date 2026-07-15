<#
.SYNOPSIS
    Runs the csp-lcrud example as SYSTEM, so it can reach the MDM WMI bridge
    (root\cimv2\mdm\dmmap), which denies access below SYSTEM.

.DESCRIPTION
    The bridge answers only the local SYSTEM account, and an elevated
    administrator prompt is not SYSTEM. This script builds the csp-lcrud
    example and runs it as SYSTEM via a transient one-shot scheduled task,
    streaming its output back. Run it from an ELEVATED PowerShell or cmd
    prompt.

    Write verbs (set / delete / cycle) change real device policy. The cycle
    verb restores the original value; set and delete do not.

.PARAMETER Verb
    The csp-lcrud subcommand: list, read, set, delete, or cycle.

.PARAMETER Value
    The integer value for `set`.

.EXAMPLE
    # From an elevated prompt, at the repo root:
    powershell -ExecutionPolicy Bypass -File scripts\Invoke-CspLcrud.ps1 read
    powershell -ExecutionPolicy Bypass -File scripts\Invoke-CspLcrud.ps1 cycle
    powershell -ExecutionPolicy Bypass -File scripts\Invoke-CspLcrud.ps1 set 2
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('list', 'read', 'set', 'delete', 'cycle', 'inspect')]
    [string]$Verb = 'read',
    [Parameter(Position = 1)]
    [string]$Value = ''
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
    Fail 'This script must run from an ELEVATED prompt (it creates a SYSTEM scheduled task).'
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Fail 'Go was not found on PATH.'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    # A prebuilt static exe is more robust than `go run` under SYSTEM, whose
    # PATH and caches differ.
    $exe = Join-Path $env:TEMP ('csplcrud-{0}.exe' -f ([guid]::NewGuid().ToString('N')))
    $log = Join-Path $env:TEMP ('csplcrud-{0}.log' -f ([guid]::NewGuid().ToString('N')))
    Write-Host "Building csp-lcrud -> $exe"
    & go build -o $exe ./examples/csp-lcrud
    if ($LASTEXITCODE -ne 0) { Fail 'go build ./examples/csp-lcrud failed.' }

    $verbArgs = @($Verb)
    if ($Verb -eq 'set') {
        if (-not $Value) { Fail 'The set verb needs a value: ... Invoke-CspLcrud.ps1 set 2' }
        $verbArgs += $Value
    }
    elseif ($Verb -eq 'inspect') {
        if (-not $Value) { Fail 'The inspect verb needs a class: ... Invoke-CspLcrud.ps1 inspect MDM_AssignedAccess' }
        $verbArgs += $Value
    }
    $quotedArgs = ($verbArgs | ForEach-Object { '"{0}"' -f $_ }) -join ' '
    $innerArgs = '/c ""{0}" {1} > "{2}" 2>&1"' -f $exe, $quotedArgs, $log

    # Register-ScheduledTask (not schtasks.exe) avoids the /TR 261-char cap.
    $taskName = 'go-bindings-wmi-csplcrud-{0}' -f ([guid]::NewGuid().ToString('N'))
    Write-Host "Running 'csp-lcrud $($verbArgs -join ' ')' as SYSTEM"
    $action = New-ScheduledTaskAction -Execute 'cmd.exe' -Argument $innerArgs
    $sysPrincipal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName $taskName -Action $action -Principal $sysPrincipal -Force | Out-Null

    $result = $null
    try {
        Start-ScheduledTask -TaskName $taskName
        $deadline = (Get-Date).AddMinutes(2)
        do {
            Start-Sleep -Milliseconds 400
            $state = (Get-ScheduledTask -TaskName $taskName).State
        } while ($state -eq 'Running' -and (Get-Date) -lt $deadline)
        $result = (Get-ScheduledTaskInfo -TaskName $taskName).LastTaskResult
    }
    finally {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }

    if (Test-Path $log) {
        Write-Host '--- csp-lcrud output ---'
        Get-Content $log | Write-Host
        Remove-Item $log -Force
    }
    Remove-Item $exe -Force -ErrorAction SilentlyContinue

    if ($result -ne 0) {
        Fail "csp-lcrud (as SYSTEM) exited with code $result. See the output above."
    }
}
finally {
    Pop-Location
}
