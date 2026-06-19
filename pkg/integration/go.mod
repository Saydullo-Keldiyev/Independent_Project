module github.com/auction-system/pkg/integration

go 1.23

require (
	github.com/auction-system/pkg/circuitbreaker v0.0.0
	github.com/auction-system/pkg/kafka v0.0.0
	github.com/auction-system/pkg/lock v0.0.0
	github.com/auction-system/pkg/logger v0.0.0
	github.com/auction-system/pkg/validation v0.0.0
	github.com/gin-gonic/gin v1.10.0
	github.com/prometheus/client_golang v1.19.1
	github.com/redis/go-redis/v9 v9.5.3
	go.uber.org/zap v1.27.0
)

replace (
	github.com/auction-system/pkg/circuitbreaker => ../circuitbreaker
	github.com/auction-system/pkg/kafka => ../kafka
	github.com/auction-system/pkg/lock => ../lock
	github.com/auction-system/pkg/logger => ../logger
	github.com/auction-system/pkg/validation => ../validation
)
