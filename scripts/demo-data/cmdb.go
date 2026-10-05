package main

import (
	"fmt"
	"strings"
)

func init() { seeders["cmdb"] = seedCMDB }

func seedCMDB(run *SeedRun) error {
	rpc, err := run.OpenRPC("127.0.0.1:9200", "cmdb.Cmdb")
	if err != nil {
		return err
	}
	defer rpc.Close()
	originalTypes, _, err := rpc.List("getCiTypeList", Row{})
	if err != nil {
		return err
	}
	attrNames := []string{"code", "name", "ip", "brand", "model", "location", "serial", "owner", "department", "environment", "os", "cpu", "memory", "disk", "rack", "zone", "purchase_date", "warranty_date", "supplier", "cost_center", "application", "purpose", "network", "gateway", "mac", "contact", "support", "version", "region", "remark"}
	attrAliases := []string{"资产编码", "资产名称", "管理IP", "品牌", "型号", "位置", "序列号", "负责人", "所属部门", "环境", "操作系统", "CPU", "内存", "磁盘", "机柜", "可用区", "采购日期", "保修截止", "供应商", "成本中心", "应用", "用途", "网络", "网关", "MAC地址", "联系人", "维保方式", "版本", "地域", "备注"}
	desired := []Row{}
	for i, name := range attrNames {
		desired = append(desired, Row{"name": "demo_asset_" + name, "alias": demoPrefix + attrAliases[i], "value_type": "text", "is_sortable": true, "is_password": false, "is_computed": false})
	}
	attrs, err := run.Ensure(rpc, "cmdb.attributes", "getAttributeList", "createAttribute", "name", Row{}, desired)
	if err != nil {
		return err
	}
	groupsWanted := []string{"计算资源", "网络资源", "存储资源", "数据服务", "应用服务", "机房设施"}
	desired = []Row{}
	for i, group := range groupsWanted {
		desired = append(desired, Row{"name": demoPrefix + group, "description": "新蜂演示模型分类", "sort": 100 + i, "icon": "ant-design:database-outlined"})
	}
	groups, err := run.Ensure(rpc, "cmdb.type_groups", "getCiTypeGroupList", "createCiTypeGroup", "name", Row{}, desired)
	if err != nil {
		return err
	}
	models := []string{"物理服务器", "虚拟服务器", "云主机", "容器节点", "计算集群", "交换机", "路由器", "防火墙", "负载均衡", "无线控制器", "块存储", "对象存储", "文件存储", "备份设备", "存储集群", "MySQL数据库", "Redis缓存", "PostgreSQL数据库", "消息队列", "搜索引擎", "业务系统", "应用服务", "API网关", "Web站点", "监控服务", "数据中心", "机房", "机柜", "配电设备", "环境传感器"}
	demoTypes := []Row{}
	if len(attrs) == 30 && len(groups) == 6 {
		desired = []Row{}
		for i, model := range models {
			desired = append(desired, Row{"name": fmt.Sprintf("newbee_demo_type_%02d", i+1), "alias": demoPrefix + model, "status": 1, "sort": 100 + i, "unique_id": rowID(attrs[0]), "show_id": rowID(attrs[1]), "default_order_attr": rowID(attrs[0]), "group_id": rowID(groups[i/5]), "icon": "ant-design:database-outlined", "is_inherited": false})
		}
		demoTypes, err = run.Ensure(rpc, "cmdb.types", "getCiTypeList", "createCiType", "name", Row{}, desired)
		if err != nil {
			return err
		}
	} else if run.Apply {
		return fmt.Errorf("CMDB demo attribute/group prerequisites missing")
	} else {
		run.Results = append(run.Results, EntityResult{Entity: "cmdb.types", Added: 30, Notes: "等待演示属性和分组ID；只读规划不创建关联"})
	}
	allTypes := []Row{}
	for _, model := range originalTypes {
		if !strings.HasPrefix(textValue(model["name"]), "newbee_demo_type_") {
			allTypes = append(allTypes, model)
		}
	}
	allTypes = append(allTypes, demoTypes...)
	demoCIs := map[string][]Row{}
	for _, model := range allTypes {
		typeID := rowID(model)
		if strings.HasPrefix(textValue(model["name"]), "newbee_demo_type_") {
			attrGroups, _, e := rpc.List("getCiTypeAttributeGroupList", Row{"type_id": typeID})
			if e != nil {
				return e
			}
			if len(attrGroups) == 0 {
				return fmt.Errorf("demo type %v has no attribute group", typeID)
			}
			current, _, e := rpc.List("getCiTypeAttributeList", Row{"type_id": typeID})
			if e != nil {
				return e
			}
			bound := map[string]bool{}
			for _, item := range current {
				bound[textValue(rowValue(item, "attribute.id"))] = true
			}
			missing := []any{}
			for _, attr := range attrs[:6] {
				if !bound[textValue(rowID(attr))] {
					missing = append(missing, rowID(attr))
				}
			}
			if run.Apply && len(missing) > 0 {
				if _, e = rpc.Call("appendAttribute", Row{"type_id": typeID, "group_id": rowID(attrGroups[0]), "attr_ids": missing}); e != nil {
					return e
				}
			}
		}
		bindings, _, e := rpc.List("getCiTypeAttributeList", Row{"type_id": typeID})
		if e != nil {
			return e
		}
		if run.Apply && strings.HasPrefix(textValue(model["name"]), "newbee_demo_type_") {
			for _, attr := range attrs[:6] {
				found := false
				for _, binding := range bindings {
					if textValue(rowValue(binding, "attribute.id")) != textValue(rowID(attr)) {
						continue
					}
					found = true
					if binding["list_show"] != true {
						if _, e = rpc.Call("changeCiTypeAttributeDefaultShow", Row{"ci_type_attribute_id": rowID(binding), "list_show": true}); e != nil {
							return e
						}
					}
				}
				if !found {
					return fmt.Errorf("demo type %v did not bind attribute %s", typeID, textValue(attr["name"]))
				}
			}
		}
		desired = []Row{}
		for n := 1; n <= run.Count; n++ {
			values := []Row{}
			for _, binding := range bindings {
				attr, ok := binding["attribute"].(map[string]any)
				if !ok {
					return fmt.Errorf("type %v missing attribute definition", typeID)
				}
				value, include := cmdbDemoValue(attr, model, n)
				if !include {
					if binding["is_required"] == true {
						return fmt.Errorf("required sensitive/unsupported attribute %s", textValue(attr["name"]))
					}
					continue
				}
				values = append(values, Row{"attr_id": rowID(attr), "attr_name": attr["name"], "attr_alias": attr["alias"], "value_type": attr["value_type"], "value": value})
			}
			key := fmt.Sprintf("newbee-demo-ci-%v-%03d", typeID, n)
			desired = append(desired, Row{"type_id": typeID, "status": 1, "available": true, "tags": []string{"newbee-demo-v1", "演示资产"}, "metadata": []Row{{"key": "demo", "value": "newbee-demo-v1"}, {"key": "demo_key", "value": key}}, "attributes": values})
		}
		items, e := run.Ensure(rpc, fmt.Sprintf("cmdb.cis.type_%v", typeID), "getCisList", "createCis", "metadata.demo_key", Row{"type_id": typeID, "with_attributes": true}, desired)
		if e != nil {
			return e
		}
		demoCIs[textValue(typeID)] = items
	}
	if !run.Apply && len(demoTypes) < 30 {
		run.Results = append(run.Results, EntityResult{Entity: "cmdb.cis.new_types", Added: (30 - len(demoTypes)) * run.Count, Notes: "每个新增模型绑定6属性并填满一页；待模型ID可用后创建"})
	}
	return seedCMDBRelationsAndPermissions(run, rpc, demoTypes, demoCIs)
}

func cmdbDemoValue(attr, model Row, n int) (string, bool) {
	name := strings.ToLower(textValue(attr["name"]))
	kind := textValue(attr["value_type"])
	typeID := number(model["id"])
	if attr["is_password"] == true || kind == "password" || kind == "reference" || attr["is_computed"] == true {
		return "", false
	}
	if choices, ok := attr["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[(n-1)%len(choices)].(map[string]any); ok {
			if v := textValue(choice["value"]); v != "" {
				return v, true
			}
		}
	}
	switch kind {
	case "int":
		return fmt.Sprint(100000 + typeID*1000 + n), true
	case "float":
		return fmt.Sprintf("%d.50", 1000+n), true
	case "boolean":
		return "true", true
	case "datetime":
		return "2026-01-15 10:00:00", true
	case "date":
		return "2026-01-15", true
	case "time":
		return "10:00:00", true
	case "json":
		return `{"demo":"newbee-demo-v1"}`, true
	case "text", "longtext", "link", "image":
	default:
		return "", false
	}
	values := map[string]string{"brand": "新蜂演示品牌", "model": "NB-DEMO-2026", "location": fmt.Sprintf("杭州演示机房-A%02d", (n-1)%6+1), "owner": fmt.Sprintf("演示负责人%02d", n), "department": "演示资产运营部", "environment": "演示环境", "os": "Linux (演示)", "cpu": "8核", "memory": "32 GB", "disk": "512 GB", "rack": fmt.Sprintf("演示机柜%02d", n), "zone": "演示可用区A", "purchase_date": "2026-01-15", "warranty_date": "2029-01-15", "supplier": "演示供应商", "cost_center": "DEMO-OPS", "application": "新蜂演示业务", "purpose": "功能展示", "network": "文档示例网络", "gateway": "192.0.2.1", "mac": fmt.Sprintf("02:00:%02x:%02x:00:%02x", typeID/256, typeID%256, n), "contact": "demo@example.invalid", "support": "演示维保", "version": "1.0-demo", "region": "杭州", "remark": "演示数据，不连接真实设备"}
	short := strings.TrimPrefix(name, "demo_asset_")
	if v, ok := values[short]; ok {
		return v, true
	}
	if strings.Contains(name, "ip") {
		return fmt.Sprintf("192.0.2.%d", ((typeID*31+n)%253)+1), true
	}
	if strings.Contains(name, "name") || strings.Contains(name, "hostname") {
		return fmt.Sprintf("%s-%03d", textValue(model["alias"]), n), true
	}
	if kind == "link" || kind == "image" {
		return "https://example.invalid/newbee-demo", true
	}
	return fmt.Sprintf("DEMO-%04d-%03d-%s", typeID, n, short), true
}

func seedCMDBRelationsAndPermissions(run *SeedRun, rpc *RPC, models []Row, cis map[string][]Row) error {
	desired := []Row{}
	for n := 1; n <= run.Count; n++ {
		desired = append(desired, Row{"name": fmt.Sprintf("演示-资产关联%02d", n), "code": fmt.Sprintf("newbee_demo_relation_%02d", n), "category": "logical", "direction": "unidirectional", "description": "演示资产拓扑关系，不执行同步", "is_enabled": true, "is_standard": false, "sort_order": 100 + n, "line_type": "solid", "display_color": "#1677ff"})
	}
	relations, err := run.Ensure(rpc, "cmdb.relation_types", "getRelationTypeList", "createRelationType", "code", Row{}, desired)
	if err != nil {
		return err
	}
	if len(models) >= 2 && len(relations) == run.Count {
		parent, child := rowID(models[0]), rowID(models[1])
		desired = []Row{}
		for _, relation := range relations {
			desired = append(desired, Row{"parent_id": parent, "child_id": child, "relation_type_id": rowID(relation), "constraint": "N:N"})
		}
		if _, err = run.Ensure(rpc, "cmdb.type_relations", "getCiTypeRelationList", "createCiTypeRelation", "relation_type_id", Row{"parent_id": parent, "child_id": child}, desired); err != nil {
			return err
		}
		sources, targets := cis[textValue(parent)], cis[textValue(child)]
		if len(sources) >= run.Count && len(targets) >= run.Count {
			for i, relation := range relations {
				source, target := rowID(sources[i]), rowID(targets[i])
				if _, err = run.Ensure(rpc, fmt.Sprintf("cmdb.ci_relations.%02d", i+1), "getCiRelationList", "createCiRelation", "relation_type_id", Row{"source_ci_id": source, "target_ci_id": target}, []Row{{"source_ci_id": source, "target_ci_id": target, "relation_type_id": rowID(relation), "discovery_source": "manual", "properties": `{"demo":"newbee-demo-v1"}`, "status": "active", "auto_sync_enabled": false, "relation_strength": "normal"}}); err != nil {
					return err
				}
			}
		}
	} else if !run.Apply {
		run.Results = append(run.Results, EntityResult{Entity: "cmdb.type_and_ci_relations", Added: run.Count * 2, Notes: "等候演示模型/CI/关系类型ID，不自动同步"})
	}
	core, err := run.OpenRPC("127.0.0.1:9100", "core.Core")
	if err != nil {
		return err
	}
	defer core.Close()
	roles, _, err := core.List("getRoleList", Row{})
	if err != nil {
		return err
	}
	roleByCode := map[string]Row{}
	for _, role := range roles {
		roleByCode[textValue(role["code"])] = role
	}
	desired = []Row{}
	for n := 1; n <= run.Count; n++ {
		role, ok := roleByCode[fmt.Sprintf("newbee_demo_role_%02d", n)]
		if !ok || len(models) == 0 {
			if run.Apply {
				return fmt.Errorf("CMDB permission requires stopped demo role %02d and demo model", n)
			}
			continue
		}
		if number(role["status"]) != 2 {
			return fmt.Errorf("demo role %02d must be disabled", n)
		}
		desired = append(desired, Row{"permission_id": fmt.Sprintf("newbee_demo_permission_%02d", n), "scope_type": "ci_type", "ci_type_id": rowID(models[(n-1)%len(models)]), "subject_type": "role", "subject_id": textValue(rowID(role)), "subject_name": role["name"], "subject_code": role["code"], "permission_type": "allow", "permission_level": "read", "operations": Row{"operations": []Row{{"operation": "read", "description": "过期演示只读权限"}}}, "status": 4, "effective_from": 1704067200, "effective_to": 1706745600, "is_temporary": true, "inheritable": false, "risk_level": "low", "description": "演示-已过期权限，不授权真实用户", "comments": "newbee-demo-v1; disabled demo role only"})
	}
	if _, err = run.Ensure(rpc, "cmdb.permissions", "getCiPermissionList", "createCiPermission", "permission_id", Row{}, desired); err != nil {
		return err
	}
	if !run.Apply && len(desired) < run.Count {
		run.Results = append(run.Results, EntityResult{Entity: "cmdb.permissions.pending_roles", Added: run.Count - len(desired), Notes: "仅等待停用演示角色与新增演示模型，状态expired且生效期已结束"})
	}
	return nil
}
