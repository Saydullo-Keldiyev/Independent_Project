package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SwaggerUI serves an embedded Swagger UI that loads our OpenAPI spec.
// Access at: http://localhost:8080/swagger/
func SwaggerUI() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, swaggerHTML)
	}
}

// SwaggerSpec serves the raw OpenAPI YAML spec
func SwaggerSpec(specContent []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/x-yaml")
		c.Data(http.StatusOK, "application/x-yaml", specContent)
	}
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Auction System API</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        .topbar { display: none; }
        .swagger-ui .info .title { font-size: 2em; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
    <script>
        SwaggerUIBundle({
            url: '/swagger/spec.yaml',
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.SwaggerUIStandalonePreset
            ],
            layout: "BaseLayout",
            defaultModelsExpandDepth: 1,
            docExpansion: "list",
            filter: true,
            tryItOutEnabled: true
        });
    </script>
</body>
</html>`
