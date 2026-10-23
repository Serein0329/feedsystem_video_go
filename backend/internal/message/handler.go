package message

import (
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/response"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Send(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "content is empty"})
		return
	}
	fromID, _ := mw.GetAccountIDByGIN(c)
	m := &Message{FromID: fromID, ToID: req.ToID, Content: req.Content, CreatedAt: time.Now()}
	if err := h.service.Send(c.Request.Context(), m); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, m)
}

func (h *Handler) List(c *gin.Context) {
	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}
	userID, _ := mw.GetAccountIDByGIN(c)
	res, err := h.service.List(c.Request.Context(), userID, req.PeerID, req.LastID, req.Limit)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if res.Messages == nil {
		res.Messages = []Message{}
	}
	response.SuccessWithPage(c, res.Messages, res.HasMore, res.NextID)
}
