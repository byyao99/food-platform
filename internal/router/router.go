package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"food-platform/internal/handlers"
	"food-platform/internal/store"
)

// New builds and configures the Gin router.
func New(s *store.Store) *gin.Engine {
	r := gin.Default()
	r.Use(cors())

	menu := handlers.NewMenuHandler(s)
	order := handlers.NewOrderHandler(s)

	// Health check (also verifies the database connection)
	r.GET("/health", func(c *gin.Context) {
		if err := s.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	{
		m := v1.Group("/menu")
		{
			m.GET("", menu.List)
			m.POST("", menu.Create)
			m.GET("/:id", menu.Get)
			m.PUT("/:id", menu.Update)
			m.DELETE("/:id", menu.Delete)
		}

		o := v1.Group("/orders")
		{
			o.GET("", order.List)
			o.POST("", order.Create)
			o.GET("/:id", order.Get)
			o.PUT("/:id", order.Update)
			o.DELETE("/:id", order.Delete)
		}
	}

	return r
}

// cors is a permissive CORS middleware suitable for local development.
func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
