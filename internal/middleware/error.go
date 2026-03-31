package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type APIResponse struct {
	Error *APIError `json:"error,omitempty"`
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		_ = c.Errors.Last().Err
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"
		message := "An internal error occurred"

		// Check for known error types
		// Will be expanded as services define custom error types
		if statusCode >= 500 {
			message = "An internal error occurred"
		}

		c.JSON(statusCode, APIResponse{
			Error: &APIError{
				Code:    errorCode,
				Message: message,
			},
		})
	}
}

func RespondError(c *gin.Context, statusCode int, code string, message string) {
	c.JSON(statusCode, APIResponse{
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
	c.Abort()
}

func RespondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

func RespondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}
