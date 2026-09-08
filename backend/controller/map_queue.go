package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	getMapQueueSnapshot   = logic.GetMapQueueSnapshot
	executeMapQueueAction = logic.ExecuteMapQueueAction
)

type mapQueueAddRequest struct {
	Map      string `json:"map" binding:"required"`
	Position string `json:"position" binding:"required"`
}

type mapQueueMapRequest struct {
	Map string `json:"map" binding:"required"`
}

type mapQueueStartRequest struct {
	Mode string `json:"mode" binding:"required"`
}

func GetMapQueueSnapshot(c *gin.Context) {
	snapshot, err := getMapQueueSnapshot()
	if err != nil {
		respondMapQueueError(c, err)
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func AddMapQueueItem(c *gin.Context) {
	var request mapQueueAddRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		FailWithError(c, http.StatusBadRequest, "地图代码和加入位置不能为空")
		return
	}
	defer LogOp(c, fmt.Sprintf("地图待办队列添加: %s (%s)", request.Map, request.Position))()

	var kind logic.MapQueueActionKind
	switch request.Position {
	case "front":
		kind = logic.MapQueueActionAddFront
	case "back":
		kind = logic.MapQueueActionAddBack
	default:
		FailWithError(c, http.StatusBadRequest, "加入位置只能是 front 或 back")
		return
	}
	performMapQueueAction(c, kind, request.Map)
}

func RemoveMapQueueItems(c *gin.Context) {
	var request mapQueueMapRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		FailWithError(c, http.StatusBadRequest, "地图代码不能为空")
		return
	}
	defer LogOp(c, "地图待办队列删除全部同名项: "+request.Map)()
	performMapQueueAction(c, logic.MapQueueActionRemove, request.Map)
}

func StartMapQueue(c *gin.Context) {
	var request mapQueueStartRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		FailWithError(c, http.StatusBadRequest, "启动模式不能为空")
		return
	}
	defer LogOp(c, "启动地图待办队列: "+request.Mode)()

	var kind logic.MapQueueActionKind
	switch request.Mode {
	case "now":
		kind = logic.MapQueueActionRun
	case "after_campaign":
		kind = logic.MapQueueActionRunAfter
	default:
		FailWithError(c, http.StatusBadRequest, "启动模式只能是 now 或 after_campaign")
		return
	}
	performMapQueueAction(c, kind, "")
}

func SkipMapQueueItem(c *gin.Context) {
	defer LogOp(c, "跳过地图待办队列当前地图")()
	performMapQueueAction(c, logic.MapQueueActionSkip, "")
}

func PauseMapQueue(c *gin.Context) {
	defer LogOp(c, "暂停地图待办队列")()
	performMapQueueAction(c, logic.MapQueueActionPause, "")
}

func ClearMapQueue(c *gin.Context) {
	defer LogOp(c, "清空并停止地图待办队列")()
	performMapQueueAction(c, logic.MapQueueActionClear, "")
}

func performMapQueueAction(c *gin.Context, kind logic.MapQueueActionKind, mapName string) {
	response, err := executeMapQueueAction(kind, mapName)
	if err != nil {
		respondMapQueueError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func respondMapQueueError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	if kind, ok := logic.GetMapQueueErrorKind(err); ok {
		switch kind {
		case logic.MapQueueErrorInvalid:
			status = http.StatusBadRequest
		case logic.MapQueueErrorRCON:
			status = http.StatusServiceUnavailable
		case logic.MapQueueErrorProtocol:
			status = http.StatusBadGateway
		case logic.MapQueueErrorRejected:
			status = http.StatusConflict
		}
	}
	FailWithError(c, status, "%v", err)
}
