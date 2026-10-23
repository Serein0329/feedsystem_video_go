package response

import "github.com/gin-gonic/gin"

type PageResult struct {
	List    any  `json:"list"`
	HasMore bool `json:"has_more"`
	NextID  uint `json:"next_id"`
}

func SuccessWithPage(c *gin.Context, list any, hasMore bool, nextID uint) {
	Success(c, PageResult{
		List:    list,
		HasMore: hasMore,
		NextID:  nextID,
	})
}
