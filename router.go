package main

import "github.com/gin-gonic/gin"

func routerEntry(router *gin.Engine) {
	v1 := router.Group("/v1")
	{
		v1.POST("/api", chinSelectCaseFunc)
	}

	v2 := router.Group("/v2")
	{
		v2.POST("/api", chinSelectFunc)
	}

	v3 := router.Group("/v3")
	{
		v3.POST("/api/deduct", DeductWalletController)
		v3.POST("/api/store", StoreWalletController)
	}

	v4 := router.Group("/v4")
	{
		v4.POST("/api/deduct", DeductWalletControllerDB)
		v4.POST("/api/store", StoreWalletControllerDB)
	}

	ws := router.Group("/ws")
	{
		ws.GET("", WsWallte)
	}

	tools := router.Group("/tools")
	{
		tools.POST("/fakedata", CreateRedisData)
		tools.POST("/fakedatadb", CreateDBData)
	}
}
