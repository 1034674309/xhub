package identity

import (
	"encoding/json"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// organizationConfigFields 严格解析组织模型范围和元数据，供组织创建、更新请求调用。
// 参数 body 为 JSON 对象；返回可区分缺省与清空的字段指针或 ErrInvalid，不产生存储副作用。
// 非数组模型、非字符串元素及非对象元数据必须拒绝，不能按空值清除已有授权；null 显式清空。
func organizationConfigFields(body map[string]any) (*[]string, *map[string]any, error) {
	var models *[]string
	var metadata *map[string]any
	if value, ok := body["models"]; ok {
		raw, err := json.Marshal(value)
		var names []string
		if err != nil || json.Unmarshal(raw, &names) != nil {
			return nil, nil, iam.ErrInvalid
		}
		names, err = iam.NormalizeOrganizationModels(names)
		if err != nil {
			return nil, nil, err
		}
		models = &names
	}
	if value, ok := body["metadata"]; ok {
		raw, err := json.Marshal(value)
		var object map[string]any
		if err != nil || json.Unmarshal(raw, &object) != nil {
			return nil, nil, iam.ErrInvalid
		}
		if object == nil {
			object = map[string]any{}
		}
		metadata = &object
	}
	return models, metadata, nil
}
