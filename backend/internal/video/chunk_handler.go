package video

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/logger"
	"feedsystem_video_go/internal/pkg/randhex"
	"feedsystem_video_go/internal/pkg/response"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
var errChunkSessionDataInvalid = errors.New("session data invalid")

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

func (h *ChunkUploadHandler) getSession(ctx context.Context, uploadID string) (*ChunkUploadSession, error) {
	if h.cache == nil {
		return nil, errChunkCacheUnavailable
	}
	b, err := h.cache.GetBytes(ctx, h.sessionKey(uploadID))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errChunkSessionNotFound
		}
		return nil, err
	}
	var s ChunkUploadSession
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, errChunkSessionDataInvalid
	}
	return &s, nil
}

func (h *ChunkUploadHandler) setSession(ctx context.Context, s *ChunkUploadSession) error {
	if h.cache == nil {
		return errChunkCacheUnavailable
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return h.cache.SetBytes(ctx, h.sessionKey(s.UploadID), b, sessionTTL)
}

func (h *ChunkUploadHandler) getLockWithRetry(ctx context.Context, uploadID string) (string, bool, error) {
	lockKey := h.cache.Key("lock:chunk_upload:%s", uploadID)
	var (
		token   string
		locked  bool
		lockErr error
	)
	for range 5 {
		token, locked, lockErr = h.cache.Lock(ctx, lockKey, 30*time.Second)
		if lockErr != nil {
			return "", false, lockErr
		}
		if locked {
			return token, true, nil
		}
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return "", false, nil
}

func (h *ChunkUploadHandler) UnLock(ctx context.Context, uploadID string, token string) error {
	lockKey := h.cache.Key("lock:chunk_upload:%s", uploadID)
	return h.cache.Unlock(ctx, lockKey, token)
}

func (h *ChunkUploadHandler) InitChunkUpload(c *gin.Context) {
	if h.cache == nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(errChunkCacheUnavailable))
		return
	}

	var req InitChunkUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	fileExt := strings.ToLower(filepath.Ext(req.Filename))
	if fileExt != ".mp4" {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "only .mp4 allowed"})
		return
	}
	if req.FileSize > MaxFilesSize {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "file size exceeds 200MB limit"})
		return
	}
	if req.ChunkSize <= 0 || req.ChunkSize > MaxChunkSize {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "invalid chunk_size"})
		return
	}
	//向上取整
	expectedChunks := int((req.FileSize + req.ChunkSize - 1) / req.ChunkSize)
	if req.TotalChunks != expectedChunks || req.TotalChunks > MaxChunks {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "invalid total_chunks"})
		return
	}
	if len(req.FileHash) > 64 {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "invalid file_hash"})
		return
	}

	// Check for existing session (resume)
	accountID, _ := mw.GetAccountIDByGIN(c)
	hashKey := h.hashKey(accountID, req.FileHash)
	existingID, err := h.cache.GetBytes(c.Request.Context(), hashKey)
	if err == nil && len(existingID) > 0 {
		session, sessErr := h.getSession(c.Request.Context(), string(existingID))
		if sessErr == nil {
			// Refresh TTL on resume
			if err := h.cache.SetBytes(c.Request.Context(), hashKey, existingID, sessionTTL); err != nil {
				logger.Info("set cache failed", slog.String("key", hashKey), slog.String("err", err.Error()))
			}
			sessionKey := h.sessionKey(session.UploadID)
			if err := h.cache.Expire(c.Request.Context(), sessionKey, sessionTTL); err != nil {
				logger.Info("set cache expired failed", slog.String("key", sessionKey), slog.String("err", err.Error()))
			}
			response.Success(c, InitChunkUploadResponse{
				UploadID:       session.UploadID,
				UploadedChunks: session.UploadedChunks(),
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
		TotalChunks:  req.TotalChunks,
		FileHash:     req.FileHash,
		UploadedBits: make([]bool, req.TotalChunks),
	}

	if err := h.setSession(c.Request.Context(), session); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}

	if err := h.cache.SetBytes(c.Request.Context(), hashKey, []byte(uploadID), sessionTTL); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}

	response.Success(c, InitChunkUploadResponse{
		UploadID:       uploadID,
		UploadedChunks: []int{},
	})
}

func (h *ChunkUploadHandler) UploadChunk(c *gin.Context) {
	if h.cache == nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(errChunkCacheUnavailable))
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxChunkSize+1<<20)

	var req UploadChunkRequest
	if err := c.ShouldBind(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, locked, err := h.getLockWithRetry(c.Request.Context(), req.UploadID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	if !locked {
		response.FailByErrCode(c, apierror.ErrTooManyRequest)
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = h.UnLock(ctx, req.UploadID, token)
	}()

	session, err := h.getSession(c.Request.Context(), req.UploadID)
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
		response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "account no permission to control this session"})
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
	if f.Size > MaxChunkSize {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "chunk too large"})
		return
	}

	chunkFile, err := f.Open()
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't open chunk: %v", err)))
		return
	}
	defer func() { _ = chunkFile.Close() }()

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
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, chunkFile); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't save chunk: %v", err)))
		return
	}

	session.UploadedBits[req.ChunkIndex] = true
	if err := h.setSession(c.Request.Context(), session); err != nil {
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

	session, err := h.getSession(c.Request.Context(), req.UploadID)
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
		response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "account no permission to control this session"})
		return
	}

	response.Success(c, ChunkStatusResponse{
		UploadID:       session.UploadID,
		TotalChunks:    session.TotalChunks,
		UploadedChunks: session.UploadedChunks(),
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

	token, locked, err := h.getLockWithRetry(c.Request.Context(), req.UploadID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	if !locked {
		response.FailByErrCode(c, apierror.ErrTooManyRequest)
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = h.UnLock(ctx, req.UploadID, token)
	}()

	session, err := h.getSession(c.Request.Context(), req.UploadID)
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
		response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "account no permission to control this session"})
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
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't create dir: %v", err)))
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
	defer func() { _ = finalFile.Close() }()

	tmpDir := filepath.Join(".run", "uploads", "tmp", req.UploadID)
	var totalSize int64
	for i := 0; i < session.TotalChunks; i++ {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("%d", i))
		cf, err := os.Open(chunkPath)
		if err != nil {
			_ = finalFile.Close()
			_ = os.Remove(finalPath)
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't open chunk: %v", err)))
			return
		}
		n, err := io.Copy(finalFile, cf)
		totalSize += n
		_ = cf.Close()
		if err != nil {
			_ = finalFile.Close()
			_ = os.Remove(finalPath)
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(fmt.Errorf("can't merge chunk: %v", err)))
			return
		}
	}
	if totalSize != session.FileSize {
		_ = finalFile.Close()
		_ = os.Remove(finalPath)
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "file size mismatch"})
		return
	}
	_ = finalFile.Close()
	// Clean up temp chunks
	_ = os.RemoveAll(tmpDir)

	// Clean up Redis session
	sessionKey := h.sessionKey(req.UploadID)
	hashKey := h.hashKey(accountID, session.FileHash)
	if err := h.cache.Del(c.Request.Context(), sessionKey); err != nil {
		logger.Info("Del cache failed", slog.String("key", sessionKey), slog.Any("error", err))
	}
	if err := h.cache.Del(c.Request.Context(), hashKey); err != nil {
		logger.Info("Del cache failed", slog.String("key", hashKey), slog.Any("error", err))
	}
	urlPath := fmt.Sprintf("/static/videos/%d/%s/%s.mp4", accountID, date, filename)

	response.Success(c, gin.H{"play_url": buildAbsoluteURL(c, urlPath)})
}
