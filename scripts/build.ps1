$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
  go test ./...
  if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
  go vet ./...
  if ($LASTEXITCODE -ne 0) { throw 'Vet failed' }
  New-Item -ItemType Directory -Force dist | Out-Null
  go build -trimpath -buildvcs=false '-ldflags=-s -w' -o dist/opencapsule.exe ./cmd/opencapsule
  if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
  Write-Host 'Build ready: dist/opencapsule.exe'
} finally { Pop-Location }
