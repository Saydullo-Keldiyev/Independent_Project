@echo off
title Auction System - Start All
echo.
echo ========================================
echo   AUCTION SYSTEM - Starting Everything
echo ========================================
echo.

echo [1/8] Starting infrastructure...
docker-compose up -d
timeout /t 10 /nobreak >nul
echo   Done.
echo.

echo [2/8] Starting User Service (8081)...
start "user-service :8081" cmd /k "cd /d d:\Go_Projects\auction-system\services\user-service && go run ./cmd/main.go"
timeout /t 3 /nobreak >nul

echo [3/8] Starting Auction Service (8083)...
start "auction-service :8083" cmd /k "cd /d d:\Go_Projects\auction-system\services\auction-service && go run ./cmd/main.go"
timeout /t 3 /nobreak >nul

echo [4/8] Starting Bid Service (8082)...
start "bid-service :8082" cmd /k "cd /d d:\Go_Projects\auction-system\services\bid-service && go run ./cmd/main.go"
timeout /t 3 /nobreak >nul

echo [5/8] Starting Payment Service (8085)...
start "payment-service :8085" cmd /k "cd /d d:\Go_Projects\auction-system\services\payment-service && go run ./cmd/main.go"
timeout /t 3 /nobreak >nul

echo [6/8] Starting Notification Service (8084)...
start "notification-service :8084" cmd /k "cd /d d:\Go_Projects\auction-system\services\notification-service && go run ./cmd/main.go"
timeout /t 3 /nobreak >nul

echo [7/8] Starting API Gateway (8080)...
start "api-gateway :8080" cmd /k "cd /d d:\Go_Projects\auction-system\api-gateway && go run ./cmd/main.go"
timeout /t 3 /nobreak >nul

echo [8/8] Starting Frontend (3000)...
start "frontend :3000" cmd /k "cd /d d:\Go_Projects\auction-system\frontend && npm run dev"
timeout /t 3 /nobreak >nul

echo.
echo ========================================
echo   ALL SERVICES STARTED!
echo ========================================
echo.
echo   Frontend:     http://localhost:3000
echo   API Gateway:  http://localhost:8080
echo   Swagger:      http://localhost:8080/swagger
echo.
echo   User:         http://localhost:8081/health
echo   Bid:          http://localhost:8082/health
echo   Auction:      http://localhost:8083/health
echo   Notification: http://localhost:8084/health
echo   Payment:      http://localhost:8085/health
echo.
echo   Stop: Close all CMD windows, then:
echo         docker-compose down
echo ========================================
pause
