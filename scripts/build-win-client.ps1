$ErrorActionPreference = "Stop"

if (!(Test-Path "bin")) {
  New-Item -ItemType Directory -Path "bin" | Out-Null
}

$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o "bin\dsp-winclient.exe" ".\cmd\winclient"
go build -o "bin\dsp-keygen.exe" ".\cmd\keygen"

Write-Host "Built:"
Write-Host " - bin\dsp-winclient.exe"
Write-Host " - bin\dsp-keygen.exe"
