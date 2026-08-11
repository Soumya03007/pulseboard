package routes

import (
	"net/http"
	"time"

	"github.com/Soumya03007/pulseboard/internal/handlers"
	"github.com/Soumya03007/pulseboard/internal/middleware"
	"github.com/Soumya03007/pulseboard/internal/repository"
	"github.com/Soumya03007/pulseboard/internal/services"
	"github.com/Soumya03007/pulseboard/pkg/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, jwtSecret string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(utils.RequestLogger(), gin.Recovery())
	router.StaticFile("/openapi.yaml", "docs/openapi.yaml")
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/ready", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	users := repository.NewUserRepository(db)
	activities := repository.NewActivityRepository(db)
	boards := repository.NewBoardRepository(db)
	auth := handlers.NewAuthHandler(services.NewAuthService(users, jwtSecret))
	profile := handlers.NewUserHandler(users, activities)
	boardHandler := handlers.NewBoardHandler(boards)
	api := router.Group("/api")
	registrationLimiter := middleware.NewIPRateLimiter(5, time.Minute)
	loginLimiter := middleware.NewIPRateLimiter(10, time.Minute)
	api.POST("/auth/register", registrationLimiter.Middleware(), auth.Register)
	api.POST("/auth/login", loginLimiter.Middleware(), auth.Login)
	meRoutes := api.Group("/me", middleware.RequireAuth(jwtSecret))
	meRoutes.GET("", profile.Me)
	meRoutes.PATCH("", profile.UpdateProfile)
	meRoutes.DELETE("", profile.DeleteProfile)
	meRoutes.GET("/activities", profile.ListActivities)
	meRoutes.POST("/activities", profile.CreateActivity)
	meRoutes.POST("/activities/complete", profile.CompleteActivity)
	boardRoutes := api.Group("/boards", middleware.RequireAuth(jwtSecret))
	boardRoutes.POST("", boardHandler.Create)
	boardRoutes.GET("", boardHandler.List)
	boardRoutes.GET("/:id", boardHandler.Get)
	boardRoutes.PATCH("/:id", boardHandler.Update)
	boardRoutes.DELETE("/:id", boardHandler.Delete)
	return router
}
