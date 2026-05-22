#!/bin/bash

# Скрипт для запуска бенчмарков с профилированием (Linux/Mac)

set -e

CPU_PROFILE=false
MEM_PROFILE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --cpu)
            CPU_PROFILE=true
            shift
            ;;
        --mem)
            MEM_PROFILE=true
            shift
            ;;
        --all)
            CPU_PROFILE=true
            MEM_PROFILE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo -e "\033[0;32mRunning benchmarks...\033[0m"

BENCH_ARGS=("-bench=." "-benchmem")

if [ "$CPU_PROFILE" = true ]; then
    BENCH_ARGS+=("-cpuprofile=cpu.prof")
    echo -e "\033[1;33mCPU profiling enabled\033[0m"
fi

if [ "$MEM_PROFILE" = true ]; then
    BENCH_ARGS+=("-memprofile=mem.prof")
    echo -e "\033[1;33mMemory profiling enabled\033[0m"
fi

BENCH_ARGS+=("./internal/...")

echo -e "\033[0;36mRunning: go test ${BENCH_ARGS[*]}\033[0m"
go test "${BENCH_ARGS[@]}"

if [ "$CPU_PROFILE" = true ] && [ -f "cpu.prof" ]; then
    echo -e "\033[0;32mCPU profile saved to cpu.prof\033[0m"
    echo -e "\033[1;33mTo analyze: go tool pprof cpu.prof\033[0m"
fi

if [ "$MEM_PROFILE" = true ] && [ -f "mem.prof" ]; then
    echo -e "\033[0;32mMemory profile saved to mem.prof\033[0m"
    echo -e "\033[1;33mTo analyze: go tool pprof mem.prof\033[0m"
fi

echo -e "\033[0;32mBenchmarks completed!\033[0m"
