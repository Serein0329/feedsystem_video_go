package http

import (
	"context"
	"feedsystem_video_go/internal/account"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/feed"
	"feedsystem_video_go/internal/message"
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/logger"
	"feedsystem_video_go/internal/pkg/response"
	"feedsystem_video_go/internal/social"
	"feedsystem_video_go/internal/sse"
	"feedsystem_video_go/internal/video"
	"feedsystem_video_go/internal/worker"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

func SetGIN(ctx context.Context, db *gorm.DB, cache *rediscache.Client, rmq *rabbitmq2.RabbitMQ) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(mw.RecoveryHandler())
	r.Use(mw.LoggerHandler())
	r.Use(mw.RequestIDHandler())
	r.Use(mw.CORSHandler())
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		response.FailByErrCode(c, apierror.ErrMethodNotAllowed)
	})
	r.NoRoute(func(c *gin.Context) {
		response.FailByErrCode(c, apierror.ErrNotFound)
	})
	r.GET("/test", func(c *gin.Context) {
		response.Success(c, "success!")
	})
	r.Static("/static", "./.run/uploads")

	// rate_limit
	loginLimiter := mw.LimitHandler(cache, "account_login", 10, time.Minute, mw.KeyByIP)
	registerLimiter := mw.LimitHandler(cache, "account_register", 5, time.Hour, mw.KeyByIP)
	refreshLimiter := mw.LimitHandler(cache, "account_refresh", 20, time.Hour, mw.KeyByIP)
	chunkLimiter := mw.LimitHandler(cache, "video_chunk", 120, time.Minute, mw.KeyByAccount)
	likeLimiter := mw.LimitHandler(cache, "video_like", 30, time.Minute, mw.KeyByAccount)
	commentLimiter := mw.LimitHandler(cache, "video_comment", 10, time.Minute, mw.KeyByAccount)
	socialLimiter := mw.LimitHandler(cache, "social_follow", 20, time.Minute, mw.KeyByAccount)

	// account
	accountRepository := account.NewAccountRepository(db)
	accountService := account.NewAccountService(accountRepository, cache)
	accountHandler := account.NewAccountHandler(accountService)
	accountGroup := r.Group("/account")
	{
		accountGroup.POST("/register", registerLimiter, accountHandler.CreateAccount)
		accountGroup.POST("/login", loginLimiter, accountHandler.Login)
		accountGroup.POST("/findByID", accountHandler.FindByID)
		accountGroup.POST("/findByUsername", accountHandler.FindByUsername)
		accountGroup.POST("/findSimilarName", accountHandler.FindSimilarName)
		accountGroup.POST("/refresh", refreshLimiter, accountHandler.Refresh)
	}
	protectedAccountGroup := accountGroup.Group("")
	protectedAccountGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedAccountGroup.POST("/logout", accountHandler.Logout)
		protectedAccountGroup.POST("/rename", accountHandler.Rename)
		protectedAccountGroup.POST("/changePassword", accountHandler.ChangePassword)
		protectedAccountGroup.POST("/uploadAvatar", accountHandler.UploadAvatar)
		protectedAccountGroup.POST("/updateProfile", accountHandler.UpdateProfile)
	}

	//video
	videoRepository := video.NewVideoRepository(db)
	timelineMQ, err := rabbitmq2.NewTimelineMQ(rmq)
	if err != nil {
		logger.Info("PopularityMQ init failed", slog.Any("error", err))
		timelineMQ = nil
	}
	videoService := video.NewVideoService(videoRepository, accountRepository, cache, timelineMQ)
	videoHandler := video.NewVideoHandler(videoService)
	chunkHandler := video.NewChunkUploadHandler(cache)
	videoGroup := r.Group("/video")
	{
		videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthorID)
		videoGroup.POST("/getByID", videoHandler.GetByID)
		videoGroup.POST("/search", videoHandler.SearchVideos)
	}
	protectedVideoGroup := videoGroup.Group("")
	protectedVideoGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedVideoGroup.POST("/uploadVideo", videoHandler.UploadVideo)
		protectedVideoGroup.POST("/deleteVideo", videoHandler.DeleteVideo)
		protectedVideoGroup.POST("/uploadCover", videoHandler.UploadCover)
		protectedVideoGroup.POST("/publish", videoHandler.PublishVideo)
		protectedVideoGroup.POST("/chunk/init", chunkLimiter, chunkHandler.InitChunkUpload)
		protectedVideoGroup.POST("/chunk/upload", chunkLimiter, chunkHandler.UploadChunk)
		protectedVideoGroup.POST("/chunk/status", chunkLimiter, chunkHandler.ChunkStatus)
		protectedVideoGroup.POST("/chunk/complete", chunkLimiter, chunkHandler.CompleteChunkUpload)
	}

	// like
	likeMQ, err := rabbitmq2.NewLikeMQ(rmq)
	if err != nil {
		logger.Info("LikeMQ init failed", slog.Any("error", err))
		likeMQ = nil
	}
	popularityMQ, err := rabbitmq2.NewPopularityMQ(rmq)
	if err != nil {
		logger.Info("PopularityMQ init failed", slog.Any("error", err))
		popularityMQ = nil
	}
	likeRepository := video.NewLikeRepository(db)
	likeService := video.NewLikeService(likeRepository, videoRepository, cache, likeMQ, popularityMQ)
	likeHandler := video.NewLikeHandler(likeService)
	protectedLikeGroup := r.Group("/like")
	protectedLikeGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedLikeGroup.POST("/like", likeLimiter, likeHandler.Like)
		protectedLikeGroup.POST("/unlike", likeLimiter, likeHandler.Unlike)
		protectedLikeGroup.POST("/isLiked", likeHandler.IsLiked)
		protectedLikeGroup.POST("/listMyLikedVideos", likeHandler.ListMyLikedVideos)
	}

	// comment
	commentMQ, err := rabbitmq2.NewCommentMQ(rmq)
	if err != nil {
		logger.Info("CommentMQ init failed", slog.Any("error", err))
		commentMQ = nil
	}
	commentRepository := video.NewCommentRepository(db)
	commentService := video.NewCommentService(commentRepository, videoRepository, cache, commentMQ, popularityMQ)
	commentHandler := video.NewCommentHandler(commentService, accountService)
	publicCommentGroup := r.Group("/comment")
	publicCommentGroup.Use(mw.JWTAuthHandler(accountRepository, cache, false))
	{
		publicCommentGroup.POST("/listAll", commentHandler.GetAllComments)
	}
	protectedCommentGroup := r.Group("/comment")
	protectedCommentGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedCommentGroup.POST("/publish", commentLimiter, commentHandler.PublishComment)
		protectedCommentGroup.POST("/delete", commentLimiter, commentHandler.DeleteComment)
	}

	// social
	socialMQ, err := rabbitmq2.NewSocialMQ(rmq)
	if err != nil {
		logger.Info("SocialMQ init failed", slog.Any("error", err))
		socialMQ = nil
	}
	socialRepository := social.NewSocialRepository(db)
	socialService := social.NewSocialService(socialRepository, accountRepository, socialMQ, cache)
	socialHandler := social.NewSocialHandler(socialService)
	socialGroup := r.Group("/social")
	protectedSocialGroup := socialGroup.Group("")
	protectedSocialGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedSocialGroup.POST("/follow", socialLimiter, socialHandler.Follow)
		protectedSocialGroup.POST("/unfollow", socialLimiter, socialHandler.Unfollow)
		protectedSocialGroup.POST("/isfollowed", socialHandler.Isfollowed)
		protectedSocialGroup.POST("/getAllFollowers", socialHandler.GetAllFollowers)
		protectedSocialGroup.POST("/getAllVloggers", socialHandler.GetAllVloggers)
		protectedSocialGroup.POST("/getCounts", socialHandler.GetCounts)
	}

	accountGroup.POST("/getProfile", func(c *gin.Context) {
		var req account.GetProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
			return
		}
		acc, err := accountService.FindByID(c.Request.Context(), req.AccountID)
		if err != nil {
			response.FailByErr(c, err)
			return
		}
		videoCount, err := videoRepository.CountByAuthor(c.Request.Context(), req.AccountID)
		if err != nil {
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
			return
		}
		totalLikes, err := videoRepository.TotalLikesByAuthor(c.Request.Context(), req.AccountID)
		if err != nil {
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
			return
		}
		followerCount, err := socialRepository.CountFollowers(c.Request.Context(), req.AccountID)
		if err != nil {
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
			return
		}
		vloggerCount, err := socialRepository.CountVloggers(c.Request.Context(), req.AccountID)
		if err != nil {
			response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
			return
		}

		response.Success(c, account.GetProfileResponse{
			Account:    *acc,
			VideoCount: videoCount, TotalLikes: totalLikes,
			FollowerCount: followerCount, VloggerCount: vloggerCount,
		})
	})

	// feed
	feedRepository := feed.NewFeedRepository(db)
	feedService := feed.NewFeedService(feedRepository, likeRepository, cache)
	feedHandler := feed.NewFeedHandler(feedService)
	publicFeedGroup := r.Group("/feed")
	publicFeedGroup.Use(mw.JWTAuthHandler(accountRepository, cache, false))
	{
		publicFeedGroup.POST("/listLatest", feedHandler.ListLatest)
		publicFeedGroup.POST("/listLikesCount", feedHandler.ListLikesCount)
		publicFeedGroup.POST("/listByPopularity", feedHandler.ListByPopularity)
		publicFeedGroup.POST("/listByTag", feedHandler.ListByTag)
	}
	protectedFeedGroup := r.Group("/feed")
	protectedFeedGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedFeedGroup.POST("/listByFollowing", feedHandler.ListByFollowing)
	}

	// message
	messageRepo := message.NewRepository(db)
	messageService := message.NewService(messageRepo, accountRepository)
	messageHandler := message.NewHandler(messageService)
	messageGroup := r.Group("/message")
	protectedMessageGroup := messageGroup.Group("")
	protectedMessageGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedMessageGroup.POST("/send", messageHandler.Send)
		protectedMessageGroup.POST("/list", messageHandler.List)
	}

	// SSE notification
	sseHub := sse.NewSSEHub(db)
	notifGroup := r.Group("/notification")
	notifGroup.Use(mw.SSEAuthHandler(accountRepository, cache))
	notifGroup.GET("/stream", sseHub.SSEHandler)
	notifGroup.POST("/list", sseHub.ListHandler)
	notifGroup.POST("/markRead", sseHub.MarkReadHandler)
	notifGroup.POST("/unreadCount", sseHub.UnreadCountHandler)

	//Worker
	if err := rabbitmq2.NewNotificationMQ(rmq); err != nil {
		logger.Info("NotificationMQ init failed", slog.Any("error", err))
	}
	go runWorkerWithRetry(ctx, "SocialWorker", rmq, func(ch *amqp.Channel) error {
		return worker.NewSocialWorker(ch, socialRepository, rabbitmq2.SocialQueue).Run(ctx)
	})
	go runWorkerWithRetry(ctx, "LikeWorker", rmq, func(ch *amqp.Channel) error {
		return worker.NewLikeWorker(ch, likeRepository, videoRepository, rabbitmq2.LikeQueue).Run(ctx)
	})
	go runWorkerWithRetry(ctx, "CommentWorker", rmq, func(ch *amqp.Channel) error {
		return worker.NewCommentWorker(ch, commentRepository, rabbitmq2.CommentQueue).Run(ctx)
	})
	for _, q := range rabbitmq2.NotificationQueues() {
		go runWorkerWithRetry(ctx, "NotificationWorker", rmq, func(ch *amqp.Channel) error {
			return worker.NewNotificationWorker(q, ch, db, sseHub).Run(ctx)
		})
	}
	if cache != nil {
		go runWorkerWithRetry(ctx, "PopularityWorker", rmq, func(ch *amqp.Channel) error {
			return worker.NewPopularityWorker(ch, cache, rabbitmq2.PopularityQueue).Run(ctx)
		})
		go runWorkerWithRetry(ctx, "TimelineWorker", rmq, func(ch *amqp.Channel) error {
			return worker.NewTimelineWorker(ch, cache, rabbitmq2.TimelineQueue).Run(ctx)
		})
	}
	return r
}

func runWorkerWithRetry(ctx context.Context, name string, rmq *rabbitmq2.RabbitMQ, fn func(*amqp.Channel) error) {
	if rmq == nil {
		logger.Info("Worker disable for rabbitmq is not initialized", slog.String("name", name))
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		ch, err := rmq.Conn.Channel()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		if err := ch.Qos(50, 0, false); err != nil {
			_ = ch.Close()
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		if err := fn(ch); err != nil && ctx.Err() == nil {
			logger.Info("Worker 消费Queue失败, 5秒后重连", slog.String("name", name), slog.Any("error", err))
		}
		_ = ch.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
