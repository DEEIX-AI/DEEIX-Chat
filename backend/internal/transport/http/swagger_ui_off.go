//go:build noswagger

package httpx

import "github.com/gin-gonic/gin"

func mountSwagger(*gin.Engine) {}
