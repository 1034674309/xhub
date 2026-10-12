package models

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sunqirui1987/xhub/internal/router"
)

// templateGroups 读取所有完整模板中的组，用于部署改名、删除时保护引用。
// 参数为模型管理宿主；返回模板声明的组和错误。无身份库表示无模板；存储或正文损坏时拒绝修改。
// 仅在调用方持有模型目录锁时读取，不加载独立全局组、不写入数据。
func templateGroups(s Host) ([]router.Group, error) {
	if s.Identity() == nil {
		return nil, nil
	}
	rows, err := s.Identity().ListRouteTemplates(context.Background())
	if err != nil {
		return nil, err
	}
	var groups []router.Group
	for _, row := range rows {
		var body map[string]any
		if err := json.Unmarshal([]byte(row.Body), &body); err != nil || body == nil {
			return nil, fmt.Errorf("template %q has unreadable settings", row.ID)
		}
		parsed, _, err := router.TemplateRouting(body)
		if err != nil {
			return nil, err
		}
		groups = append(groups, parsed...)
	}
	return groups, nil
}

// checkRoutingGroupRemoval 检查公开模型最后部署是否仍被某模板组引用。
// 参数为宿主与公开模型名；返回引用或存储错误，供删除/改名接口在持锁时拒绝修改，无副作用。
func checkRoutingGroupRemoval(s Host, name string) error {
	groups, err := templateGroups(s)
	if err != nil {
		return err
	}
	for _, group := range groups {
		for _, member := range group.Models {
			if member == name {
				return fmt.Errorf("remove model %q from template routing group %q first", name, group.Name)
			}
		}
	}
	return nil
}

// checkRoutingGroupName 防止新增或改名的公开模型遮蔽任何模板的组名。
// 参数为宿主和新公开名；返回冲突或存储错误。管理接口持锁调用，不修改模板或部署。
func checkRoutingGroupName(s Host, name string) error {
	groups, err := templateGroups(s)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.Name == name {
			return fmt.Errorf("model name %q shadows a template routing group", name)
		}
	}
	return nil
}
