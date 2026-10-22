package video

import (
	"feedsystem_video_go/internal/account"
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	service        *CommentService
	accountService *account.AccountService
}

func NewCommentHandler(service *CommentService, accountService *account.AccountService) *CommentHandler {
	return &CommentHandler{service: service, accountService: accountService}
}
func (h *CommentHandler) PublishComment(c *gin.Context) {
	var req PublishCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "comment content is empty"})
		return
	}
	authorId, _ := mw.GetAccountIDByGIN(c)
	username, _ := mw.GetUsernameByGIN(c)
	comment := &Comment{
		Username: username,
		VideoID:  req.VideoID,
		AuthorID: authorId,
		Content:  req.Content,
	}
	if err := h.service.Publish(c.Request.Context(), comment); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *CommentHandler) DeleteComment(c *gin.Context) {
	var req DeleteCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, _ := mw.GetAccountIDByGIN(c)
	if err := h.service.Delete(c.Request.Context(), req.CommentID, accountID); err != nil {
		response.FailByErr(c, err)
		return
	}

	response.Success(c, nil)
}

func (h *CommentHandler) GetAllComments(c *gin.Context) {
	var req GetAllCommentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}
	if _, err := mw.GetAccountIDByGIN(c); err != nil {
		req.Limit = 10
		req.LastID = 0
	}
	res, err := h.service.GetAll(c.Request.Context(), req.VideoID, req.LastID, req.Limit)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if res.Comments == nil {
		res.Comments = []Comment{}
	}
	response.SuccessWithPage(c, res.Comments, res.HasMore, res.LastID)
}
