package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"food-platform/internal/auth"
	"food-platform/internal/handlers"
	"food-platform/internal/middleware"
	"food-platform/internal/models"
	"food-platform/internal/store"
)

// New builds and configures the Gin router. am signs and verifies bearer
// tokens; log receives one structured record per request.
func New(s *store.Store, am *auth.Manager, log *slog.Logger) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Logger(log), gin.Recovery(), cors())

	menu := handlers.NewMenuHandler(s)
	order := handlers.NewOrderHandler(s)
	authH := handlers.NewAuthHandler(s, am)
	user := handlers.NewUserHandler(s)

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
		a := v1.Group("/auth")
		{
			a.POST("/register", authH.Register)
			a.POST("/login", authH.Login)
			a.PUT("/password", middleware.RequireAuth(am), authH.ChangePassword)
		}

		// User management: admin-only. Provision accounts, list them, change
		// roles, and delete them.
		u := v1.Group("/users", middleware.RequireRole(am, models.RoleAdmin))
		{
			u.GET("", user.List)
			u.POST("", user.Create)
			u.PUT("/:id/role", user.UpdateRole)
			u.PUT("/:id/password", user.ResetPassword)
			u.DELETE("/:id", user.Delete)
		}

		// Menu: reads are public; writes require an admin.
		m := v1.Group("/menu")
		{
			m.GET("", menu.List)
			m.GET("/:id", menu.Get)
			m.POST("", middleware.RequireRole(am, models.RoleAdmin), menu.Create)
			m.PUT("/:id", middleware.RequireRole(am, models.RoleAdmin), menu.Update)
			m.DELETE("/:id", middleware.RequireRole(am, models.RoleAdmin), menu.Delete)
		}

		// Orders: any authenticated user may place and view an order; managing
		// the queue (listing all, status changes, deletion) requires staff/admin.
		o := v1.Group("/orders")
		{
			o.POST("", middleware.RequireAuth(am), order.Create)
			o.GET("/:id", middleware.RequireAuth(am), order.Get)
			o.GET("", middleware.RequireRole(am, models.RoleStaff, models.RoleAdmin), order.List)
			o.PUT("/:id", middleware.RequireRole(am, models.RoleStaff, models.RoleAdmin), order.Update)
			o.DELETE("/:id", middleware.RequireRole(am, models.RoleStaff, models.RoleAdmin), order.Delete)
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
