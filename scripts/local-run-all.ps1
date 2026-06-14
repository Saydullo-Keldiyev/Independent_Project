$ErrorActionPreference = "Continue"
$ROOT = "d:\Go_Projects\auction-system"
Set-Location $ROOT

Write-Host ""
Write-Host "=== AUCTION SYSTEM - Starting All Services ===" -ForegroundColor Cyan
Write-Host ""

Write-Host "[1/3] Checking infrastructure..." -ForegroundColor Yellow
docker exec auction-postgres pg_isready -U auction -d auction_db 2>$null | Out-Null
if ($LASTEXITCODE -eq 0) { Write-Host "  OK PostgreSQL" -ForegroundColor Green }
docker exec auction-redis redis-cli ping 2>$null | Out-Null
if ($LASTEXITCODE -eq 0) { Write-Host "  OK Redis" -ForegroundColor Green }
Write-Host "  OK Kafka" -ForegroundColor Green
Write-Host ""

Write-Host "[2/3] Launching services..." -ForegroundColor Yellow

$services = @(
    @{ Name = "user-service"; Path = "services\user-service"; Port = "8081" },
    @{ Name = "auction-service"; Path = "services\auction-service"; Port = "8083" },
    @{ Name = "bid-service"; Path = "services\bid-service"; Port = "8082" },
    @{ Name = "payment-service"; Path = "services\payment-service"; Port = "8085" },
    @{ Name = "notification-service"; Path = "services\notification-service"; Port = "8084" },
    @{ Name = "api-gateway"; Path = "api-gateway"; Port = "8080" }
)

foreach ($svc in $services) {
    $svcPath = Join-Path $ROOT $svc.Path
    $cmd = "Set-Location '$svcPath'; go run ./cmd/main.go"
    Start-Process powershell -ArgumentList "-NoExit","-Command",$cmd
    Write-Host ("  Started: " + $svc.Name + " -> http://localhost:" + $svc.Port) -ForegroundColor Green
    Start-Sleep -Seconds 3
}

Write-Host ""
Write-Host "[3/3] All services launched!" -ForegroundColor Green
Write-Host ""
Write-Host "  API Gateway:  http://localhost:8080" -ForegroundColor White
Write-Host "  Swagger UI:   http://localhost:8080/swagger" -ForegroundColor White
Write-Host "  Health:       http://localhost:8080/health" -ForegroundColor White
Write-Host ""
Write-Host "  Stop: Close all PowerShell windows, then: docker-compose down" -ForegroundColor Yellow
