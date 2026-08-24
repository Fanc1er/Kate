package search

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册搜索相关路由。
func RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/search")
	{
		api.GET("/global", func(c *gin.Context) {
			keyword := c.Query("q")
			if keyword == "" {
				c.JSON(400, gin.H{"error": "keyword is required"})
				return
			}
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			if page < 1 {
				page = 1
			}

			idx := Instance()
			docs, total, err := idx.Search(c.Request.Context(), keyword, page, 20)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{
				"keyword": keyword,
				"total":   total,
				"page":    page,
				"items":   docs,
			})
		})
	}
}
