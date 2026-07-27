// Package handler 提供 HTTP 请求处理函数。
package handler

import (
	"context"
	"strings"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const tongjiCallbackOrigin = "https://app.tongji.edu.cn"

// 同济开放平台授权范围
var tongjiAuthorizationScopes = []string{
	"openid",
	"rt_onetongji_school_calendar_all_term_calendar",
	"dc_student_work_info_stipend",
	"rt_onetongji_undergraduate_score",
	"dc_sep_auth_student_accommodation_info",
	"user",
	"dc_student_work_info_honorary_title",
	"dc_student_work_info_competition_winners",
	"dc_lib_lib_access_control",
	"dc_door_school_access_control",
	"rt_teaching_info_sports_test_health",
	"dc_user_student_info",
	"rt_onetongji_student_timetable",
	"rt_onetongji_cet_score",
	"rt_teaching_info_sports_test_data",
	"dc_lib_lend_info",
	"rt_onetongji_school_calendar_current_term_calendar",
	"dc_card_card_history_flow",
	"dc_student_work_info_scholarship",
	"dc_lib_lend_info_all",
}

// tongjiTokenRequest 表示回调页面提交的授权码。
type tongjiTokenRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// tongjiBearerToken 表示仅供回调页面使用的短期 Bearer access token。
type tongjiBearerToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// TongjiAuthorize 将浏览器重定向至同济统一认证和授权页面。
func TongjiAuthorize(ctx context.Context, c *app.RequestContext) {
	client, err := tongjiapi.NewFromEnv()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "Tongji Open Platform is not configured"})
		return
	}
	state, err := client.CreateState()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "cannot create OAuth state"})
		return
	}
	redirectURL, err := client.AuthorizationURL(state, tongjiAuthorizationScopes)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "cannot create authorization URL"})
		return
	}
	c.Redirect(consts.StatusFound, []byte(redirectURL))
}

// TongjiExchangeToken 使用 callback.html 提交的 code 换取 Bearer access token。
func TongjiExchangeToken(ctx context.Context, c *app.RequestContext) {
	setTongjiCallbackCORS(c)
	var request tongjiTokenRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "request body must be valid JSON"})
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	request.State = strings.TrimSpace(request.State)
	if request.Code == "" || request.State == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "code and state are required"})
		return
	}

	client, err := tongjiapi.NewFromEnv()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "Tongji Open Platform is not configured"})
		return
	}
	if validateErr := client.ValidateState(request.State); validateErr != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "invalid or expired OAuth state"})
		return
	}

	token, err := client.ExchangeAuthorizationCode(ctx, request.Code)
	if err != nil {
		c.JSON(consts.StatusBadGateway, utils.H{"error": "exchange authorization code failed"})
		return
	}
	c.JSON(consts.StatusOK, tongjiBearerToken{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		ExpiresIn:   token.ExpiresIn,
		Scope:       token.Scope,
	})
}

// TongjiExchangeTokenOptions 响应 callback.html 的跨域预检请求。
func TongjiExchangeTokenOptions(_ context.Context, c *app.RequestContext) {
	setTongjiCallbackCORS(c)
	c.Status(consts.StatusNoContent)
}

// setTongjiCallbackCORS 仅允许登记的 callback 页面读取换取的 access token。
func setTongjiCallbackCORS(c *app.RequestContext) {
	if string(c.Request.Header.Get("Origin")) != tongjiCallbackOrigin {
		return
	}
	c.Response.Header.Set("Access-Control-Allow-Origin", tongjiCallbackOrigin)
	c.Response.Header.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type")
	c.Response.Header.Set("Vary", "Origin")
}
