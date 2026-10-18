package apierror

import "net/http"

// 通用错误 10xxx
var (
	ErrBadRequest       = New(http.StatusBadRequest, 10001, "请求参数错误")
	ErrUnauthorized     = New(http.StatusUnauthorized, 10002, "未登录")
	ErrForbidden        = New(http.StatusForbidden, 10003, "权限不足")
	ErrNotFound         = New(http.StatusNotFound, 10004, "资源不存在")
	ErrMethodNotAllowed = New(http.StatusMethodNotAllowed, 10005, "请求方法不允许")
	ErrTooManyRequest   = New(http.StatusTooManyRequests, 10006, "请求过于频繁")
	ErrInternalServer   = New(http.StatusInternalServerError, 10007, "服务器内部错误")
)

// 用户模块 20xxx
var (
	ErrUserExist            = New(http.StatusConflict, 20001, "用户已存在")
	ErrUserNotFound         = New(http.StatusNotFound, 20002, "用户不存在")
	ErrUserPasswordWrong    = New(http.StatusBadRequest, 20003, "密码错误")
	ErrUserNoFieldsToUpdate = New(http.StatusBadRequest, 20004, "没有更新的内容")
)

// 视频模块 30xxx
var (
	ErrVideoNotFound = New(http.StatusNotFound, 30001, "视频不存在")
)

// 评论模块 40xxx
var (
	ErrCommentNotFound = New(http.StatusNotFound, 40001, "评论不存在")
)

// 社交模块 50xxx
var (
	ErrSocialNotFollowSelf = New(http.StatusBadRequest, 50002, "不能关注自己")
)
