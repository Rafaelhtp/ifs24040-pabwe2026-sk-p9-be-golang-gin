package response

import "github.com/gin-gonic/gin"

const (
	StatusSuccess = "success"
	StatusFail    = "fail"
	StatusError   = "error"
)

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func resolveData(data []any) any {
	if len(data) > 0 {
		return data[0]
	}
	return nil
}

func Success(c *gin.Context, statusCode int, message string, data ...any) {
	c.JSON(statusCode, Response{
		Status:  StatusSuccess,
		Message: message,
		Data:    resolveData(data),
	})
}

func Fail(c *gin.Context, statusCode int, message string, data ...any) {
	c.JSON(statusCode, Response{
		Status:  StatusFail,
		Message: message,
		Data:    resolveData(data),
	})
}

func Error(c *gin.Context, statusCode int, message string, data ...any) {
	c.JSON(statusCode, Response{
		Status:  StatusError,
		Message: message,
		Data:    resolveData(data),
	})
}

func AbortFail(c *gin.Context, statusCode int, message string, data ...any) {
	c.AbortWithStatusJSON(statusCode, Response{
		Status:  StatusFail,
		Message: message,
		Data:    resolveData(data),
	})
}

func AbortError(c *gin.Context, statusCode int, message string, data ...any) {
	c.AbortWithStatusJSON(statusCode, Response{
		Status:  StatusError,
		Message: message,
		Data:    resolveData(data),
	})
}
