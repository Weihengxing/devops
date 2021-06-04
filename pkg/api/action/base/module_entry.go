package base

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/yametech/devops/pkg/api"
	"strconv"
)

func (b *baseServer) CreateModuleEntry(c *gin.Context) {
	user := c.Request.Header.Get("x-wrapper-username")
	uuid := c.Query("uuid")
	page, err := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	if err != nil {
		api.ResponseError(c, errors.New("page need int type"))
		return
	}
	pageSize, err := strconv.ParseInt(c.DefaultQuery("page_size", "10"), 10, 64)
	if err != nil {
		api.ResponseError(c, errors.New("pageSize need int type"))
		return
	}
	response, err := b.CreateEntry(user, uuid, page, pageSize)
	if err != nil {
		api.ResponseError(c, err)
		return
	}

	api.ResponseSuccess(c, response, "")
}

func (b *baseServer) DeleteModuleEntry(c *gin.Context) {
	user := c.Request.Header.Get("x-wrapper-username")
	uuid := c.Query("uuid")
	page, err := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	if err != nil {
		api.ResponseError(c, errors.New("page need int type"))
		return
	}
	pageSize, err := strconv.ParseInt(c.DefaultQuery("page_size", "10"), 10, 64)
	if err != nil {
		api.ResponseError(c, errors.New("pageSize need int type"))
		return
	}
	response, err := b.DeleteEntry(user, uuid, page, pageSize)
	if err != nil {
		api.ResponseError(c, err)
		return
	}
	api.ResponseSuccess(c, response, "")
}

func (b *baseServer) QueryModuleEntry(c *gin.Context) {
	user := c.Request.Header.Get("x-wrapper-username")
	page, err := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	if err != nil {
		api.ResponseError(c, errors.New("page need int type"))
		return
	}
	pageSize, err := strconv.ParseInt(c.DefaultQuery("page_size", "10"), 10, 64)
	if err != nil {
		api.ResponseError(c, errors.New("pageSize need int type"))
		return
	}
	response, err := b.QueryEntry(user, page, pageSize)
	if err != nil {
		api.ResponseError(c, err)
		return
	}
	api.ResponseSuccess(c, response, "")
}
