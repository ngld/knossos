$global:ProgressPreference = 'SilentlyContinue'

$ToolRoot = "${PSScriptRoot}\.tools"
$GoRoot = "${PSScriptRoot}\third_party\go"
$Go = "${GoRoot}\bin\go.exe"

function Get-Go($Source, $Target) {
    Write-Host "Downloading Go toolchain..."
    New-Item $Target -ItemType Directory -ea 0 | Out-Null

    $Zip = "${PSScriptRoot}\go.zip"
    $Client = New-Object System.Net.WebClient
    $Client.DownloadFile($Source, $Zip)

    Expand-Archive -Path $Zip -DestinationPath $Target
    Remove-Item -Path $Zip
}

if (!(Test-Path -Path $Go -ea 0)) {
    Get-Go -Source 'https://golang.org/dl/go1.16.3.windows-amd64.zip' -Target (Split-Path -Path $GoRoot)

    if (!(Test-Path -Path $Go -ea 0)) {
        Write-Host "Could not find or fetch Go!"
        Write-Host ""
        Write-Host "Please install the Go toolchain, or fix the previous error"
        Write-Host "and try again."
        Write-Host ""
        Exit 1
    }
}

if (Test-Path -Path "${ToolRoot}\tool.exe.rebuild" -ea 0){
    Remove-Item -Path "${ToolRoot}\tool.exe"
    Remove-Item -Path "${ToolRoot}\tool.exe.rebuild"
}

if (!(Test-Path -Path "${ToolRoot}\tool.exe" -ea 0)) {
    Push-Location
    Set-Location -Path 'packages\build-tools'
    Invoke-Expression "${Go} build -o '${ToolRoot}\tool.exe'"
    Pop-Location
}

Invoke-Expression "${ToolRoot}\tool.exe task ${Args}"