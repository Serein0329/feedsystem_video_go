package response

import "github.com/gin-gonic/gin"

type PageResult struct {
	List    any  `json:"list"`
	HasMore bool `json:"has_more"`
	LastID  uint `json:"last_id"`
}

func SuccessWithPage(c *gin.Context, list any, hasMore bool, lastID uint) {
	Success(c, PageResult{
		List:    list,
		HasMore: hasMore,
		LastID:  lastID,
	})
}
