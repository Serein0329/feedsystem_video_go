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
	"log"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

func SetGIN(db *gorm.DB, cache *rediscache.Client, rmq *rabbitmq2.RabbitMQ) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Printf("SetTrustedProxies failed: %v", err)
	}
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
	likeLimiter := mw.LimitHandler(cache, "video_like", 30, time.Minute, mw.KeyByAccount)
	commentLimiter := mw.LimitHandler(cache, "video_comment", 10, time.Minute, mw.KeyByAccount)
	socialLimiter := mw.LimitHandler(cache, "social_follow", 20, time.Minute, mw.KeyByAccount)
	chunkLimiter := mw.LimitHandler(cache, "video_chunk", 120, time.Minute, mw.KeyByAccount)

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
	videoService := video.NewVideoService(videoRepository, cache, timelineMQ)
	videoHandler := video.NewVideoHandler(videoService, accountService)
	chunkHandler := video.NewChunkUploadHandler(cache)
	videoGroup := r.Group("/video")
	{
		videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthorID)
		videoGroup.POST("/getDetail", videoHandler.GetDetail)
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
	commentGroup := r.Group("/comment")
	{
		commentGroup.POST("/listAll", commentHandler.GetAllComments)
	}
	protectedCommentGroup := commentGroup.Group("")
	protectedCommentGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedCommentGroup.POST("/publish", commentLimiter, commentHandler.PublishComment)
		protectedCommentGroup.POST("/delete", commentLimiter, commentHandler.DeleteComment)
	}

	// social
	socialMQ, err := rabbitmq2.NewSocialMQ(rmq)
	if err != nil {
		log.Printf("SocialMQ init failed: %v", err)
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
	feedGroup := r.Group("/feed")
	feedGroup.Use(mw.JWTAuthHandler(accountRepository, cache, false))
	{
		feedGroup.POST("/listLatest", feedHandler.ListLatest)
		feedGroup.POST("/listLikesCount", feedHandler.ListLikesCount)
		feedGroup.POST("/listByPopularity", feedHandler.ListByPopularity)
		feedGroup.POST("/listByTag", feedHandler.ListByTag)
	}
	protectedFeedGroup := feedGroup.Group("")
	protectedFeedGroup.Use(mw.JWTAuthHandler(accountRepository, cache, true))
	{
		protectedFeedGroup.POST("/listByFollowing", feedHandler.ListByFollowing)
	}

	// message
	messageRepo := message.NewRepository(db)
	messageService := message.NewService(messageRepo)
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
	notifGroup.Use(mw.SSEAuthHandler())
	notifGroup.GET("/stream", sseHub.SSEHandler)
	notifGroup.POST("/list", sseHub.ListHandler)
	notifGroup.POST("/markRead", sseHub.MarkReadHandler)
	notifGroup.POST("/unreadCount", sseHub.UnreadCountHandler)

	//Worker
	worker.StartOutboxConsumer("video.timeline.events", cache, rmq)
	go func() {
		if rmq != nil {
			if err := rabbitmq2.NewNotificationMQ(rmq); err != nil {
				log.Printf("notification topics init failed: %v", err)
			}
			if err := worker.NewNotificationWorker(rmq, db, sseHub).Run(context.Background()); err != nil {
				log.Printf("NotificationWorker Run err: %v", err.Error())
			}
		} else {
			log.Printf("Notification SSE disabled")
		}
	}()
	go runWorkerWithRetry("SocialWorker", rmq, func(ch *amqp.Channel) error {
		return worker.NewSocialWorker(ch, socialRepository, "social.events").Run(context.Background())
	})
	go runWorkerWithRetry("LikeWorker", rmq, func(ch *amqp.Channel) error {
		return worker.NewLikeWorker(ch, likeRepository, videoRepository, "like.events").Run(context.Background())
	})
	go runWorkerWithRetry("CommentWorker", rmq, func(ch *amqp.Channel) error {
		return worker.NewCommentWorker(ch, commentRepository, videoRepository, "comment.events").Run(context.Background())
	})
	if cache != nil {
		go runWorkerWithRetry("PopularityWorker", rmq, func(ch *amqp.Channel) error {
			return worker.NewPopularityWorker(ch, cache, "video.popularity.events").Run(context.Background())
		})
	}
	return r
}

func runWorkerWithRetry(name string, rmq *rabbitmq2.RabbitMQ, fn func(*amqp.Channel) error) {
	if rmq == nil {
		return
	}
	for {
		ch, err := rmq.Conn.Channel()
		if err != nil {
			log.Printf("%s: 创建 Channel 失败: %v, 5秒后重试", name, err)
			time.Sleep(5 * time.Second)
			continue
		}
		if err := ch.Qos(50, 0, false); err != nil {
			log.Printf("%s: QoS 设置失败: %v, 5秒后重试", name, err)
			_ = ch.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("%s started, consuming", name)
		if err := fn(ch); err != nil {
			log.Printf("%s: %v, 5秒后重连...", name, err)
		}
		_ = ch.Close()
		time.Sleep(5 * time.Second)
	}
}
