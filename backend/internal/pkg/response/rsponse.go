package response

import (
	"errors"
	"feedsystem_video_go/internal/pkg/apierror"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int    `json:"code"`           // 业务状态码，0 表示成功
	Msg  string `json:"msg"`            // 提示信息
	Data any    `json:"data,omitempty"` // 业务数据，无数据时省略该字段
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "ok",
		Data: data,
	})
}

func SuccessWithMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  msg,
		Data: data,
	})
}

func Fail(c *gin.Context, httpStatus int, code int, msg string) {
	c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
	})
}

func FailByErr(c *gin.Context, err error) {
	if apierr, ok := errors.AsType[*apierror.ApiError](err); ok {
		FailByErrCode(c, apierr)
		return
	}
	FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
}

func FailByErrCode(c *gin.Context, err *apierror.ApiError) {
	if err.HttpStatus == http.StatusInternalServerError {
		_ = c.Error(err)
	}
	c.JSON(err.HttpStatus, Response{
		Code: err.Code,
		Msg:  err.Msg,
	})
}

func FailWithData(c *gin.Context, httpStatus int, code int, msg string, data any) {
	c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

func FailWithDataByErrCode(c *gin.Context, err *apierror.ApiError, data any) {
	if err.HttpStatus == http.StatusInternalServerError {
		_ = c.Error(err)
	}
	c.JSON(err.HttpStatus, Response{
		Code: err.Code,
		Msg:  err.Msg,
		Data: data,
	})
}
