# Run locally after reviewing the sources. Requires Git and GitHub CLI logged in.
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
  if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { throw 'Install GitHub CLI (gh) first.' }
  gh auth status
  if ($LASTEXITCODE -ne 0) { throw 'Run gh auth login first.' }
  if (-not (Test-Path .git)) { git init; git branch -M main }
  git add .
  git commit -m 'feat: OpenCapsule v1 preview with persistent containers and archives'
  if ($LASTEXITCODE -ne 0) { Write-Host 'No changes to commit or commit failed.' }
  gh repo create PdB333/opencapsule --public --source . --remote origin --push
  if ($LASTEXITCODE -ne 0) { throw 'GitHub repository creation/push failed. Resolve then retry manually.' }
  Write-Host 'Published.'
} finally { Pop-Location }
