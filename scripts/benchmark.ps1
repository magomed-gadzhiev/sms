# Скрипт для запуска бенчмарков с профилированием (PowerShell для Windows)

param(
    [switch]$CPUProfile,
    [switch]$MemProfile,
    [switch]$All
)

Write-Host "Running benchmarks..." -ForegroundColor Green

$benchArgs = @("-bench=.", "-benchmem")

if ($CPUProfile -or $All) {
    $benchArgs += @("-cpuprofile=cpu.prof")
    Write-Host "CPU profiling enabled" -ForegroundColor Yellow
}

if ($MemProfile -or $All) {
    $benchArgs += @("-memprofile=mem.prof")
    Write-Host "Memory profiling enabled" -ForegroundColor Yellow
}

$benchArgs += @("./internal/...")

Write-Host "Running: go test $($benchArgs -join ' ')" -ForegroundColor Cyan
& go test @benchArgs

if ($CPUProfile -or $All) {
    if (Test-Path "cpu.prof") {
        Write-Host "CPU profile saved to cpu.prof" -ForegroundColor Green
        Write-Host "To analyze: go tool pprof cpu.prof" -ForegroundColor Yellow
    }
}

if ($MemProfile -or $All) {
    if (Test-Path "mem.prof") {
        Write-Host "Memory profile saved to mem.prof" -ForegroundColor Green
        Write-Host "To analyze: go tool pprof mem.prof" -ForegroundColor Yellow
    }
}

Write-Host "Benchmarks completed!" -ForegroundColor Green
