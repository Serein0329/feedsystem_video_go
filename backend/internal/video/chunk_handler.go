package video

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/randhex"
	"feedsystem_video_go/internal/pkg/response"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const sessionTTL = 24 * time.Hour

var errChunkCacheUnavailable = errors.New("chunk upload requires redis")
var errChunkSessionNotFound = errors.New("session not found")

type ChunkUploadHandler struct {
	cache *rediscache.Client
}

func NewChunkUploadHandler(cache *rediscache.Client) *ChunkUploadHandler {
	return &ChunkUploadHandler{cache: cache}
}

func (h *ChunkUploadHandler) sessionKey(uploadID string) string {
	return h.cache.Key("chunk_upload:%s", uploadID)
}

func (h *ChunkUploadHandler) hashKey(accountID uint, fileHash string) string {
	return h.cache.Key("chunk_upload_hash:%d:%s", accountID, fileHash)
}

func (h *ChunkUploadHandler) getSession(ctx *gin.Context, uploadID string) (*ChunkUploadSession, error) {
	if h.cache == nil {
		return nil, errChunkCacheUnavailable
	}
	b, err := h.cache.GetBytes(ctx.Request.Context(), h.sessionKey(uploadID))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errChunkSessionNotFound
		}
		return nil, err
	}
	var s ChunkUploadSession
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("invalid session data")
	}
	return &s, nil
}

func (h *ChunkUploadHandler) setSession(ctx *gin.Context, s *ChunkUploadSession) error {
	if h.cache == nil {
		return errChunkCacheUnavailable
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return h.cache.SetBytes(ctx.Request.Context(), h.sessionKey(s.UploadID), b, sessionTTL)
}

func (h *ChunkUploadHandler) InitChunkUpload(c *gin.Context) {
	if h.cache == nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(errChunkCacheUnavailable))
		return
	}

	var req InitChunkUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"err": err.Error()})
		return
	}

	accountID, _ := mw.GetAccountIDByGIN(c)

	fileExt := strings.ToLower(filepath.Ext(req.Filename))
	switch fileExt {
	case ".mp4":
	default:
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "only .mp4 allowed"})
		return
	}

	const maxSize = 200 << 20
	if req.FileSize > maxSize {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "file size exceeds 200MB limit"})
		return
	}

	// Check for existing session (resume)
	hashKey := h.hashKey(accountID, req.FileHash)
	existingID, err := h.cache.GetBytes(c.Request.Context(), hashKey)
	if err == nil && len(existingID) > 0 {
		session, sessErr := h.getSession(c, string(existingID))
		if sessErr == nil {
			// Refresh TTL on resume
			_ = h.cache.SetBytes(c.Request.Context(), hashKey, existingID, sessionTTL)
			_ = h.setSession(c, session)
			response.Success(c, gin.H{
				"upload_id":       session.UploadID,
				"uploaded_chunks": session.UploadedChunks(),
			})
			return
		}
	}

	id, err := randhex.New(16)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	uploadID := id + fmt.Sprintf("%d", time.Now().UnixNano())
	session := &ChunkUploadSession{
		UploadID:     uploadID,
		AccountID:    accountID,
		Filename:     req.Filename,
		FileSize:     req.FileSize,
		ChunkSize:    req.ChunkSize,
		TotalChunks:  req.TotalChunks,
		FileHash:     req.FileHash,
		UploadedBits: make([]bool, req.TotalChunks),
	}

	if err := h.setSession(c, session); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}

	if err := h.cache.SetBytes(c.Request.Context(), hashKey, []byte(uploadID), sessionTTL); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}

	response.Success(c, gin.H{
		"upload_id":       uploadID,
		"uploaded_chunks": []int{},
	})
}

func (h *ChunkUploadHandler) UploadChunk(c *gin.Context) {
	if h.cache == nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(errChunkCacheUnavailable))
		return
	}

	var req UploadChunkRequest
	if err := c.ShouldBind(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.getSession(c, req.UploadID)
	if err != nil {
		if errors.Is(err, errChunkSessionNotFound) {
			response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		} else {
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		}
		return
	}

	accountID, _ := mw.GetAccountIDByGIN(c)
	if session.AccountID != accountID {
		response.FailByErrCode(c, apierror.ErrUnauthorized)
		return
	}

	if req.ChunkIndex >= session.TotalChunks {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "chunk index out of range"})
		return
	}

	if session.UploadedBits[req.ChunkIndex] {
		response.Success(c, gin.H{"chunk_index": req.ChunkIndex})
		return
	}

	f, err := c.FormFile("file")
	if err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "missing file"})
		return
	}

	chunkFile, err := f.Open()
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't open chunk: %v", err)))
		return
	}
	defer chunkFile.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, chunkFile); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't hash chunk: %v", err)))
		return
	}
	actualHash := fmt.Sprintf("%x", hash.Sum(nil))

	if actualHash != req.ChunkHash {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "chunk hash mismatch", "expected": req.ChunkHash, "actual": actualHash})
		return
	}

	tmpDir := filepath.Join(".run", "uploads", "tmp", req.UploadID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't create tmp dir: %v", err)))
		return
	}

	chunkPath := filepath.Join(tmpDir, fmt.Sprintf("%d", req.ChunkIndex))
	// io.Copy(hash, chunkFile)已经让游标在文件末尾了
	if _, err := chunkFile.Seek(0, io.SeekStart); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't seek chunk: %v", err)))
		return
	}

	dst, err := os.Create(chunkPath)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't create chunk file: %v", err)))
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, chunkFile); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't save chunk: %v", err)))
		return
	}

	session.UploadedBits[req.ChunkIndex] = true
	if err := h.setSession(c, session); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't update session: %v", err)))
		return
	}

	response.Success(c, gin.H{"chunk_index": req.ChunkIndex})
}

func (h *ChunkUploadHandler) ChunkStatus(c *gin.Context) {
	if h.cache == nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(errChunkCacheUnavailable))
		return
	}

	var req ChunkStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.getSession(c, req.UploadID)
	if err != nil {
		if errors.Is(err, errChunkSessionNotFound) {
			response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		} else {
			response.FailWithDataByErrCode(c, apierror.ErrInternalServer, err)
		}
		return
	}

	accountID, _ := mw.GetAccountIDByGIN(c)
	if session.AccountID != accountID {
		response.FailByErrCode(c, apierror.ErrUnauthorized)
		return
	}

	response.Success(c, gin.H{
		"upload_id":       session.UploadID,
		"uploaded_chunks": session.UploadedChunks(),
		"total_chunks":    session.TotalChunks,
	})
}

func (h *ChunkUploadHandler) CompleteChunkUpload(c *gin.Context) {
	if h.cache == nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(errChunkCacheUnavailable))
		return
	}

	var req CompleteChunkUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.getSession(c, req.UploadID)
	if err != nil {
		if errors.Is(err, errChunkSessionNotFound) {
			response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		} else {
			response.FailWithDataByErrCode(c, apierror.ErrInternalServer, err)
		}
		return
	}

	accountID, _ := mw.GetAccountIDByGIN(c)
	if session.AccountID != accountID {
		response.FailByErrCode(c, apierror.ErrUnauthorized)
		return
	}

	if !session.IsComplete() {
		missing := 0
		for _, uploaded := range session.UploadedBits {
			if !uploaded {
				missing++
			}
		}
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{
			"error":     "not all chunks uploaded",
			"missing":   missing,
			"completed": session.TotalChunks - missing,
			"total":     session.TotalChunks,
		})
		return
	}

	date := time.Now().Format("20060102")
	relDir := filepath.Join("videos", fmt.Sprintf("%d", accountID), date)
	root := filepath.Join(".run", "uploads")
	absDir := filepath.Join(root, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't create  dir: %v", err)))
		return
	}

	filename, err := randhex.New(16)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't generate filename: %v", err)))
		return
	}
	finalPath := filepath.Join(absDir, filename+".mp4")

	finalFile, err := os.Create(finalPath)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't create final file: %v", err)))
		return
	}
	defer finalFile.Close()

	tmpDir := filepath.Join(".run", "uploads", "tmp", req.UploadID)
	for i := 0; i < session.TotalChunks; i++ {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("%d", i))
		cf, err := os.Open(chunkPath)
		if err != nil {
			_ = finalFile.Close()
			_ = os.Remove(finalPath)
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't open chunk: %v", err)))
			return
		}
		_, err = io.Copy(finalFile, cf)
		_ = cf.Close()
		if err != nil {
			_ = finalFile.Close()
			_ = os.Remove(finalPath)
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't merge chunk: %v", err)))
			return
		}
	}
	_ = finalFile.Close()

	// Clean up temp chunks
	_ = os.RemoveAll(tmpDir)

	// Clean up Redis session
	_ = h.cache.Del(c.Request.Context(), h.sessionKey(req.UploadID))
	_ = h.cache.Del(c.Request.Context(), h.hashKey(accountID, session.FileHash))

	urlPath := fmt.Sprintf("/static/videos/%d/%s/%s.mp4", accountID, date, filename)
	playURL := buildAbsoluteURL(c, urlPath)

	response.Success(c, gin.H{"play_url": playURL})
}
