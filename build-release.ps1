# build-release.ps1
# Script de automatización de releases locales para Scoop y empaquetado.

$Version = "1.0.0"
$ZipFile = "gentle-skills-bridge-windows-amd64.zip"
$ManifestFile = "gentle-skills-bridge.json"

Write-Host "1. Compilando binario de producción..." -ForegroundColor Cyan
go build -ldflags="-s -w" -o gentle-skills-bridge.exe .
if ($LASTEXITCODE -ne 0) {
    Write-Error "Falló la compilación del código en Go."
    exit 1
}
Write-Host "Compilación exitosa." -ForegroundColor Green

Write-Host "2. Creando archivo ZIP para Scoop..." -ForegroundColor Cyan
if (Test-Path $ZipFile) { 
    Remove-Item $ZipFile -Force 
}

# Empaquetamos el ejecutable y el README en el ZIP
Compress-Archive -Path gentle-skills-bridge.exe, README.md -DestinationPath $ZipFile
if ($LASTEXITCODE -ne 0) {
    Write-Error "No se pudo crear el archivo ZIP."
    exit 1
}
Write-Host "Archivo ZIP creado: $ZipFile" -ForegroundColor Green

Write-Host "3. Calculando hash SHA-256 del ZIP..." -ForegroundColor Cyan
$Hash = (Get-FileHash -Path $ZipFile -Algorithm SHA256).Hash.ToLower()
Write-Host "Hash SHA-256 generado: $Hash" -ForegroundColor Green

Write-Host "4. Actualizando manifiesto de Scoop ($ManifestFile)..." -ForegroundColor Cyan
if (Test-Path $ManifestFile) {
    $ManifestContent = Get-Content $ManifestFile -Raw | ConvertFrom-Json
    
    # Actualizamos los campos necesarios
    $ManifestContent.version = $Version
    $ManifestContent.architecture."64bit".hash = $Hash
    
    # Convertimos de vuelta a JSON formateado con indentación
    $UpdatedJson = $ManifestContent | ConvertTo-Json -Depth 100
    
    # Forzar escritura en UTF-8 sin BOM (estándar para Scoop)
    [System.IO.File]::WriteAllText((Resolve-Path $ManifestFile), $UpdatedJson)
    
    Write-Host "¡Manifiesto de Scoop actualizado con el nuevo hash!" -ForegroundColor Green
} else {
    Write-Warning "No se encontró el archivo de manifiesto $ManifestFile."
}
