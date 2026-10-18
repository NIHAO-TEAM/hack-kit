# Creates .claude\skills -> ..\.agents\skills (junction) in the repo root and in each part.
# Junctions need neither admin rights nor developer mode.
# .claude/skills is in .gitignore: the link is local, git only stores .agents/skills.
# ASCII only: Windows PowerShell 5.1 reads BOM-less UTF-8 as ANSI.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

foreach ($dir in @($root, (Join-Path $root 'frontend'), (Join-Path $root 'backend'))) {
    $source = Join-Path $dir '.agents\skills'
    if (-not (Test-Path $source)) { continue }
    $claude = Join-Path $dir '.claude'
    if (-not (Test-Path $claude)) { New-Item -ItemType Directory -Path $claude | Out-Null }
    $link = Join-Path $claude 'skills'
    if (Test-Path $link) {
        $item = Get-Item $link -Force
        if ($item.LinkType) { Write-Output "ok: $link"; continue }
        Write-Warning "skip: $link exists and is not a link, check it manually"
        continue
    }
    New-Item -ItemType Junction -Path $link -Target $source | Out-Null
    Write-Output "created: $link -> $source"
}
