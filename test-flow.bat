@echo off
echo ========================================
echo   AUCTION SYSTEM — Full Flow Test
echo ========================================
echo.

echo [1] Health checks...
curl -s http://localhost:8080/health
echo.
curl -s http://localhost:8081/health
echo.
curl -s http://localhost:8082/health
echo.
curl -s http://localhost:8083/health
echo.
echo.

echo [2] Register bidder...
curl -s -X POST http://localhost:8080/api/v1/auth/register -H "Content-Type: application/json" -d "{\"username\":\"bidder1\",\"email\":\"bidder@test.com\",\"password\":\"Test12345!\",\"first_name\":\"John\",\"last_name\":\"Bidder\",\"role\":\"bidder\"}"
echo.
echo.

echo [3] Register seller...
curl -s -X POST http://localhost:8080/api/v1/auth/register -H "Content-Type: application/json" -d "{\"username\":\"seller1\",\"email\":\"seller@test.com\",\"password\":\"Test12345!\",\"first_name\":\"Jane\",\"last_name\":\"Seller\",\"role\":\"seller\"}"
echo.
echo.

echo [4] Login as seller...
curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"email\":\"seller@test.com\",\"password\":\"Test12345!\"}"
echo.
echo.

echo ========================================
echo   Test complete! Check responses above.
echo ========================================
pause
