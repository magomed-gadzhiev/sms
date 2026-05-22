# PowerShell скрипт для генерации gRPC кода из proto файлов

# Проверка наличия protoc
$protocPath = Get-Command protoc -ErrorAction SilentlyContinue
if (-not $protocPath) {
    Write-Host "Ошибка: protoc не установлен" -ForegroundColor Red
    Write-Host "Установите protoc: https://grpc.io/docs/protoc-installation/" -ForegroundColor Yellow
    exit 1
}

# Проверка наличия плагинов
$goPath = (go env GOPATH)
$protocGenGo = Join-Path $goPath "bin\protoc-gen-go.exe"
$protocGenGoGrpc = Join-Path $goPath "bin\protoc-gen-go-grpc.exe"

if (-not (Test-Path $protocGenGo)) {
    Write-Host "Установка protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
}

if (-not (Test-Path $protocGenGoGrpc)) {
    Write-Host "Установка protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
}

# Список всех proto файлов для генерации
$protoFiles = @(
    @{path="api\proto\sms.proto"; output="api\proto\smsv1"},
    @{path="api\proto\auth\auth.proto"; output="api\proto\authv1"},
    @{path="api\proto\messaging\messaging.proto"; output="api\proto\messagingv1"},
    @{path="api\proto\routing\routing.proto"; output="api\proto\routingv1"},
    @{path="api\proto\provider\provider.proto"; output="api\proto\providerv1"},
    @{path="api\proto\client\client.proto"; output="api\proto\clientv1"},
    @{path="api\proto\analytics\analytics.proto"; output="api\proto\analyticsv1"},
    @{path="api\proto\billing\billing.proto"; output="api\proto\billingv1"}
)

$errors = @()

foreach ($proto in $protoFiles) {
    $outputDir = $proto.output
    $protoPath = $proto.path
    
    # Создание директории для сгенерированного кода
    if (-not (Test-Path $outputDir)) {
        New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
    }
    
    # Проверка существования proto файла
    if (-not (Test-Path $protoPath)) {
        Write-Host "Предупреждение: файл $protoPath не найден, пропускаем..." -ForegroundColor Yellow
        continue
    }
    
    Write-Host "Генерация кода из $protoPath..."
    
    # Генерация кода
    protoc `
        --go_out=$outputDir `
        --go_opt=paths=source_relative `
        --go-grpc_out=$outputDir `
        --go-grpc_opt=paths=source_relative `
        --proto_path=api\proto `
        $protoPath
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ $protoPath - успешно" -ForegroundColor Green
    } else {
        Write-Host "  ✗ $protoPath - ошибка" -ForegroundColor Red
        $errors += $protoPath
    }
}

if ($errors.Count -eq 0) {
    Write-Host "`nГенерация всех proto файлов завершена успешно!" -ForegroundColor Green
    exit 0
} else {
    Write-Host "`nОшибка при генерации следующих файлов:" -ForegroundColor Red
    foreach ($err in $errors) {
        Write-Host "  - $err" -ForegroundColor Red
    }
    exit 1
}
