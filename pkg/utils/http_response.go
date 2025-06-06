package utils

import (
	"github.com/gin-gonic/gin"
)

type JSONResponse struct{
	Success bool		`json:"success"`
	Message string		`json:"message"`
	Data interface{}	`json:"data,omitempty"`
	Error interface{}	`json:"error,omitempty"`
}

func ResponseWithSuccess(
	c *gin.Context,
	statusCode int,
	message string,
	data interface{},
){
	c.JSON(statusCode, JSONResponse{
		Success: true,
		Message: message,
		Data: data,
	})
}

func ResponseWithError(
	c *gin.Context,
	statusCode int,
	message string,
	errorDetails interface{},
){
	c.JSON(statusCode, JSONResponse{
		Success: false,
		Message: message,
		Error: errorDetails,
	})
}