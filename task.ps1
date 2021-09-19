$ToolRoot = "${PSScriptRoot}\.tools"
$GoRoot = "${PSScriptRoot}\third_party\go"

$env:Path = "${GoRoot}\bin;${env:Path}"
$global:ProgressPreference = 'SilentlyContinue'

function Get-Go($Source, $Target) {
    Write-Host "Downloading Go toolchain..."
    New-Item $Target -ItemType Directory -ea 0 | Out-Null

    $Zip = "${PSScriptRoot}\go.zip"
    (New-Object System.Net.WebClient).DownloadFile($Source, $Zip)

    Expand-Archive -Path $Zip -DestinationPath $Target
    Remove-Item -Path $Zip

    if (!(Test-Path -Path "${Target}\go\bin\go.exe" -ea 0)) {
        Write-Host "Could not find or fetch Go ${GoVersion}! $Target"
        Write-Host ""
        Write-Host "Please install the Go toolchain, or fix the previous error"
        Write-Host "and try again."
        Write-Host ""
        Exit 1
    }
}

$GoVersion = Get-Content "${PSScriptRoot}/.go-version"
if (!(Get-Command 'go.exe' -ea 0)) {
    Get-Go -Source "https://golang.org/dl/go${GoVersion}.windows-amd64.zip" -Target (Split-Path -Path $GoRoot)
}

$CurrentGoVersion = go version
$CurrentGoVersion = $CurrentGoVersion.Split(" ")[2].Substring(2)
if (!($CurrentGoVersion -eq $GoVersion)) {
    Write-Host "Go version mismatch detected! Expected $GoVersion but found $CurrentGoVersion!"

    if (Test-Path -Path "${GoRoot}") {
        Write-Host "Deleting old Go installation (in third_party)..."
        Remove-Item -Recurse -Path "${GoRoot}"
    }

    Get-Go -Source "https://golang.org/dl/go${GoVersion}.windows-amd64.zip" -Target (Split-Path -Path $GoRoot)
}

if (!(Test-Path -Path "${ToolRoot}\tool.exe" -ea 0)) {
    Write-Host "Building build-tools..."

    Push-Location
    Set-Location -Path 'packages\build-tools'
    Invoke-Expression "go.exe build -o '${ToolRoot}\tool.exe'"
    Pop-Location
}

Invoke-Expression "${ToolRoot}\tool.exe task ${Args}"
$code = $LastExitCode
Remove-Item -Path "${ToolRoot}\tool.exe.old.*"
Exit $code
