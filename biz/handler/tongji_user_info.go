package handler

import (
	"context"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type userBasicInfoClient interface {
	GetUserBasicInfo(ctx context.Context, accessToken string) (*tongjiapi.UserBasicInfo, error)
}

var newUserBasicInfoClient = func() (userBasicInfoClient, error) {
	return tongjiapi.NewFromEnv()
}

// TongjiUserBasicInfo 使用当前 Bearer access token 查询用户基础信息。
func TongjiUserBasicInfo(ctx context.Context, c *app.RequestContext) {
	accessToken, err := platformauth.ExtractBearerToken(string(c.Request.Header.Get("Authorization")))
	if err != nil {
		c.JSON(consts.StatusUnauthorized, utils.H{"error": "valid access token is required"})
		return
	}
	client, err := newUserBasicInfoClient()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "Tongji Open Platform is not configured"})
		return
	}
	info, err := client.GetUserBasicInfo(ctx, accessToken)
	if err != nil {
		c.JSON(consts.StatusBadGateway, utils.H{"error": "get user basic info failed"})
		return
	}
	if info == nil {
		c.JSON(consts.StatusBadGateway, utils.H{"error": "get user basic info failed"})
		return
	}
	c.JSON(consts.StatusOK, info)
}

var _ userBasicInfoClient = (*tongjiapi.Client)(nil)
