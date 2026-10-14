package models

import (
	"context"
	"strings"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/util"
)

// AllowsModel 为模型目录与推理返回统一授权判断；参数为宿主、上下文、主体、预览团队及公开模型名。
// 主凭证始终允许，其他主体按组织、团队、项目和密钥交集判断；查询失败或空名称返回 false，无写入副作用。
func AllowsModel(s Host, ctx context.Context, p *auth.Principal, teamID, alias string) bool {
	util.Trace("evaluate model access")
	alias = strings.TrimSpace(alias)
	if p == nil || alias == "" {
		return false
	}
	if p.IsMaster() {
		return true
	}
	names, err := effectiveModels(s, ctx, p, teamID)
	if err != nil {
		util.Error("model access failed kind=%s user=%s err=%v", p.Kind, p.UserID, err)
		return false
	}
	if names == nil {
		return true
	}
	for _, name := range names {
		if name == alias {
			return true
		}
	}
	return false
}

// effectiveModels 返回调用方模型交集，供目录和推理的统一权限判断调用。
// 参数 s/ctx/p 为宿主、上下文及主体，teamID 仅供平台管理员预览指定团队；普通会话始终读取本人团队。
// 返回 nil 表示未限制，非 nil 空列表表示无权限，错误必须拒绝；只读，避免通过省略或伪造 team_id 绕过组织上限。
func effectiveModels(s Host, ctx context.Context, p *auth.Principal, teamID string) ([]string, error) {
	if s == nil || s.Identity() == nil {
		return nil, nil
	}
	if p.Key != nil {
		return s.Identity().AllowedModelsForKey(ctx, p.Key)
	}
	if !p.PlatformAdmin() {
		return s.Identity().AllowedModelsForUser(ctx, p.UserID)
	}
	if teamID != "" {
		return s.Identity().AllowedModelsForTeam(ctx, teamID)
	}
	return nil, nil
}
