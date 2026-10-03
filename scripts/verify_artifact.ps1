# verify_artifact.ps1 — run the Windows release binary where it can actually run.
#
# PowerShell rather than bash: Git Bash on the Windows runner is not what a
# player uses, and a zip that only unpacks under MSYS proves little.
param([string]$DistDir = "dist")
$ErrorActionPreference = "Stop"

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$zip = Get-ChildItem -Path $DistDir -Filter "rabbitrun_*_windows_$arch.zip" | Select-Object -First 1
if (-not $zip) { throw "no windows/$arch zip in $DistDir" }

$work = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
Expand-Archive -Path $zip.FullName -DestinationPath $work -Force
foreach ($f in @("rabbitrun.exe", "LICENSE", "NOTICE.md", "README.md")) {
  if (-not (Test-Path (Join-Path $work $f))) { throw "$($zip.Name) does not contain $f" }
}

$exe = Join-Path $work "rabbitrun.exe"
$p = Start-Process -FilePath $exe -ArgumentList "--version" -NoNewWindow -Wait -PassThru
if ($p.ExitCode -ne 0) { throw "rabbitrun.exe --version exited with $($p.ExitCode)" }
Write-Host "ok: $($zip.Name) runs"
