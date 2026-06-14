@echo off
REM ═══════════════════════════════════════════════════════════════════════════
REM  AUCTION SYSTEM — Local Start (Windows)
REM ═══════════════════════════════════════════════════════════════════════════
REM
REM  Step 1: docker-compose up -d (infra)
REM  Step 2: Run each service in separate terminal
REM
REM  Usage: scripts\local-start.bat
REM ═══════════════════════════════════════════════════════════════════════════

echo.
echo ╔═══════════════════════════════════════════════════════╗
echo ║       AUCTION SYSTEM — Local Development             ║
echo ╚═══════════════════════════════════════════════════════╝
echo.

REM ── Step 1: Start infrastructure ────────────────────────────────────────────
echo [1/3] Starting infrastructure (Postgres, Redis, Kafka)...
docker-compose up -d
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Docker Compose failed. Is Docker Desktop running?
    pause
    exit /b 1
)

echo.
echo [2/3] Waiting for services to be healthy...
timeout /t 15 /nobreak > nul

REM Check postgres
docker exec auction-postgres pg_isready -U auction -d auction_db > nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo   ✓ PostgreSQL ready
) else (
    echo   ✗ PostgreSQL not ready — waiting more...
    timeout /t 10 /nobreak > nul
)

REM Check redis
docker exec auction-redis redis-cli ping > nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo   ✓ Redis ready
) else (
    echo   ✗ Redis not ready
)

echo   ✓ Kafka starting (takes ~30s)

echo.
echo [3/3] Infrastructure ready!
echo.
echo ═══════════════════════════════════════════════════════════════
echo  Now start services in SEPARATE terminals:
echo ═══════════════════════════════════════════════════════════════
echo.
echo  Terminal 1 (API Gateway):
echo    cd services\user-service ^& go run ./cmd/main.go
echo.
echo  Terminal 2 (User Service):
echo    cd api-gateway ^& go run ./cmd/main.go
echo.
echo  Terminal 3 (Auction Service):
echo    cd services\auction-service ^& go run ./cmd/main.go
echo.
echo  Terminal 4 (Bid Service):
echo    cd services\bid-service ^& go run ./cmd/main.go
echo.
echo  Terminal 5 (Payment Service):
echo    cd services\payment-service ^& go run ./cmd/main.go
echo.
echo  Terminal 6 (Notification Service):
echo    cd services\notification-service ^& go run ./cmd/main.go
echo.
echo ═══════════════════════════════════════════════════════════════
echo  Ports:
echo    API Gateway:          http://localhost:8080
echo    User Service:         http://localhost:8081
echo    Bid Service:          http://localhost:8082
echo    Auction Service:      http://localhost:8083
echo    Notification Service: http://localhost:8084
echo    Payment Service:      http://localhost:8085
echo ═══════════════════════════════════════════════════════════════
echo.
echo  Health check:
echo    curl http://localhost:8080/health
echo.
echo  Stop infra:
echo    docker-compose down
echo.
pause
