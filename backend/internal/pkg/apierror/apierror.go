package apierror

type ApiError struct {
	HttpStatus int    `json:"-"`
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	Internal   string `json:"-"`
}

func (e *ApiError) Error() string {
	if e.Internal != "" {
		return e.Internal
	}
	return e.Msg
}

func New(httpStatus, code int, msg string) *ApiError {
	return &ApiError{
		HttpStatus: httpStatus,
		Code:       code,
		Msg:        msg,
	}
}

func (e *ApiError) WithInternal(err error) *ApiError {
	newErr := *e
	newErr.Internal = err.Error()
	return &newErr
}
