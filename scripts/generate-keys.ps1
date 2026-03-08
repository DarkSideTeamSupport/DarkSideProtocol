#!/usr/bin/env pwsh
# Скрипт генерации новых ключей для DarkSideProtocol

param(
    [string]$OutputDir = "configs",
    [switch]$UpdateConfigs
)

Write-Host "Генерация ключей..." -ForegroundColor Cyan

# Генерируем ключи через keygen
$keygenOutput = go run ./cmd/keygen | ConvertFrom-Json

$serverPrivate = $keygenOutput.server.private_key
$serverPublic = $keygenOutput.server.public_key
$clientPrivate = $keygenOutput.client.private_key
$clientPublic = $keygenOutput.client.public_key

Write-Host "Server Public Key:  $serverPublic" -ForegroundColor Yellow
Write-Host "Server Private Key: $serverPrivate" -ForegroundColor Gray
Write-Host "Client Public Key:  $clientPublic" -ForegroundColor Yellow
Write-Host "Client Private Key: $clientPrivate" -ForegroundColor Gray

# Генерируем PSK
$psk = -join ((65..90) + (97..122) + (48..57) | Get-Random -Count 32 | ForEach-Object {[char]$_})
Write-Host "Pre-Shared Key:     $psk" -ForegroundColor Green

if ($UpdateConfigs) {
    # Обновляем server.json
    $serverConfig = Get-Content "$OutputDir/server.json" | ConvertFrom-Json
    $serverConfig.pre_shared_key = $psk
    $serverConfig.server_private_key = $serverPrivate
    $serverConfig.server_public_key = $serverPublic
    $serverConfig | ConvertTo-Json -Depth 10 | Set-Content "$OutputDir/server.json"
    Write-Host "Updated: $OutputDir/server.json" -ForegroundColor Cyan

    # Обновляем client.json
    $clientConfig = Get-Content "$OutputDir/client.json" | ConvertFrom-Json
    $clientConfig.pre_shared_key = $psk
    $clientConfig.server_public_key = $serverPublic
    $clientConfig.client_private_key = $clientPrivate
    $clientConfig.client_public_key = $clientPublic
    $clientConfig | ConvertTo-Json -Depth 10 | Set-Content "$OutputDir/client.json"
    Write-Host "Updated: $OutputDir/client.json" -ForegroundColor Cyan

    # Обновляем winclient.json
    $winclientConfig = Get-Content "$OutputDir/winclient.json" | ConvertFrom-Json
    $winclientConfig.pre_shared_key = $psk
    $winclientConfig.server_public_key = $serverPublic
    $winclientConfig.client_private_key = $clientPrivate
    $winclientConfig.client_public_key = $clientPublic
    $winclientConfig | ConvertTo-Json -Depth 10 | Set-Content "$OutputDir/winclient.json"
    Write-Host "Updated: $OutputDir/winclient.json" -ForegroundColor Cyan

    Write-Host "`nКонфиги обновлены! Не забудьте скопировать PSK и ключи на сервер/клиенты." -ForegroundColor Green
} else {
    Write-Host "`nЗапустите с флагом -UpdateConfigs для автоматического обновления конфигов" -ForegroundColor Yellow
    Write-Host "Пример: .\scripts\generate-keys.ps1 -UpdateConfigs" -ForegroundColor Gray
}
