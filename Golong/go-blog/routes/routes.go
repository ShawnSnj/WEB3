package routes

import (
	"go-blog/controllers"
	"go-blog/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	r.POST("/register", controllers.Register)
	r.POST("/login", controllers.Login)

	auth := r.Group("/")
	auth.Use(middleware.JWTAuth())
	{
		auth.POST("/posts", controllers.CreatePost)
		auth.POST("/posts/:id/comments", controllers.CreateComment)
		auth.PUT("/posts/:id", controllers.UpdatePost)
		auth.DELETE("/posts/:id", controllers.DeletePost)
	}

	r.GET("/posts", controllers.GetAllPosts)
	r.GET("/posts/:id", controllers.GetPostByID)
	r.GET("/posts/:id/comments", controllers.GetCommentsByPostID)
	return r
}
