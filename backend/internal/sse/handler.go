package sse

import (
	"encoding/json"
	"errors"
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/response"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SSEHub struct {
	mu      sync.RWMutex
	clients map[uint][]chan *Notification
	db      *gorm.DB
}

func NewSSEHub(db *gorm.DB) *SSEHub {
	return &SSEHub{clients: make(map[uint][]chan *Notification), db: db}
}

func (h *SSEHub) Push(userID uint, n *Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	chs, ok := h.clients[userID]
	if !ok {
		return
	}
	for _, ch := range chs {
		select {
		case ch <- n:
		default:
		}
	}
}

func (h *SSEHub) Subscribe(userID uint) chan *Notification {
	ch := make(chan *Notification, 20)
	h.mu.Lock()
	h.clients[userID] = append(h.clients[userID], ch)
	h.mu.Unlock()
	return ch
}

func (h *SSEHub) Unsubscribe(userID uint, ch chan *Notification) {
	h.mu.Lock()
	defer h.mu.Unlock()
	chs := h.clients[userID]
	for i, c := range chs {
		if c == ch {
			chs = append(chs[:i], chs[i+1:]...)
			if len(chs) == 0 {
				delete(h.clients, userID)
			} else {
				h.clients[userID] = chs
			}
			close(c)
			return
		}
	}
}

func (h *SSEHub) SSEHandler(c *gin.Context) {
	userID, _ := mw.GetAccountIDByGIN(c)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	ch := h.Subscribe(userID)
	defer h.Unsubscribe(userID, ch)

	flusher, _ := c.Writer.(http.Flusher)

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case n, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(n)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(c.Writer, "event: notification\ndata: %s\n\n", b); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		case <-time.After(30 * time.Second):
			_, _ = fmt.Fprintf(c.Writer, ": keepalive\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func (h *SSEHub) ListHandler(c *gin.Context) {
	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}
	userID, _ := mw.GetAccountIDByGIN(c)
	var notifications []Notification
	query := h.db.WithContext(c.Request.Context()).Where("recipient_id = ?", userID)
	if req.LastID > 0 {
		query = query.Where("id < ?", req.LastID)
	}
	if err := query.Order("id desc").Limit(req.Limit).Find(&notifications).Error; err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	if notifications == nil {
		notifications = []Notification{}
	}
	if len(notifications) > 0 {
		req.LastID = notifications[len(notifications)-1].ID
	}
	response.SuccessWithPage(c, notifications, len(notifications) == req.Limit, req.LastID)
}

func (h *SSEHub) MarkReadHandler(c *gin.Context) {
	userID, _ := mw.GetAccountIDByGIN(c)

	var req struct {
		ID *uint `json:"id" binding:"omitempty,min=1"`
	}
	//!errors.Is(err, io.EOF)可以让不输入请求体的请求通过
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	var err error
	if req.ID != nil {
		err = h.db.WithContext(c.Request.Context()).Model(&Notification{}).Where("id = ? AND recipient_id = ?", *req.ID, userID).Update("is_read", true).Error
	} else {
		err = h.db.WithContext(c.Request.Context()).Model(&Notification{}).Where("recipient_id = ?", userID).Update("is_read", true).Error
	}
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	response.Success(c, nil)
}

func (h *SSEHub) UnreadCountHandler(c *gin.Context) {
	userID, _ := mw.GetAccountIDByGIN(c)

	var count int64
	if err := h.db.WithContext(c.Request.Context()).Model(&Notification{}).Where("recipient_id = ? AND is_read = ?", userID, false).Count(&count).Error; err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	response.Success(c, gin.H{"count": count})
}
