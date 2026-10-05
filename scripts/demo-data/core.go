package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func init() { seeders["core"] = seedCore }

func chosenID(rows []Row, index int) any {
	if len(rows) == 0 {
		return 1
	} // Read-only planning never creates dependent rows.
	return rows[index%len(rows)]["id"]
}

func seedCore(run *SeedRun) error {
	rpc, err := run.OpenRPC("127.0.0.1:9100", "core.Core")
	if err != nil {
		return err
	}
	defer rpc.Close()
	departments := []string{"资产运营中心", "基础设施部", "云平台部", "信息安全部", "应用运维部", "网络管理部", "数据库管理部", "业务支持部", "研发一部", "研发二部", "质量保障部", "数据治理部", "采购管理部", "财务管理部", "人力资源部", "客户服务部", "项目管理部", "技术支持部", "杭州园区", "上海园区", "北京园区", "深圳园区", "成都园区", "武汉园区", "南京园区", "西安园区", "济南园区", "广州园区", "天津园区", "重庆园区"}
	rows := []Row{}
	for i := 0; i < run.Count; i++ {
		rows = append(rows, Row{"name": demoPrefix + departments[i%len(departments)], "status": 1, "sort": i + 100, "parent_id": 1, "ancestors": "0,1", "leader": fmt.Sprintf("演示负责人%02d", i+1), "email": fmt.Sprintf("dept%02d@example.invalid", i+1), "remark": "新蜂平台演示组织，不关联真实人员"})
	}
	depts, err := run.Ensure(rpc, "core.departments", "getDepartmentList", "createDepartment", "name", Row{}, rows)
	if err != nil {
		return err
	}
	rows = []Row{}
	for i := 0; i < run.Count; i++ {
		rows = append(rows, Row{"name": fmt.Sprintf("%s%s岗位", demoPrefix, departments[i%len(departments)]), "code": fmt.Sprintf("newbee_demo_position_%02d", i+1), "status": 1, "sort": i + 100, "dept_id": chosenID(depts, i), "remark": "演示岗位"})
	}
	positions, err := run.Ensure(rpc, "core.positions", "getPositionList", "createPosition", "code", Row{}, rows)
	if err != nil {
		return err
	}
	rows = []Row{}
	for i := 0; i < run.Count; i++ {
		rows = append(rows, Row{"name": fmt.Sprintf("%s%s观察员", demoPrefix, departments[i%len(departments)]), "code": fmt.Sprintf("newbee_demo_role_%02d", i+1), "status": 2, "sort": i + 100, "default_router": "/workspace", "remark": "停用的演示角色，不授予真实菜单或接口权限", "menu_ids": []any{}})
	}
	roles, err := run.Ensure(rpc, "core.roles", "getRoleList", "createRole", "code", Row{}, rows)
	if err != nil {
		return err
	}
	rows = []Row{}
	for i := 0; i < run.Count; i++ {
		secret := make([]byte, 32)
		if _, err = rand.Read(secret); err != nil {
			return err
		}
		rows = append(rows, Row{"username": fmt.Sprintf("newbee_demo_user_%02d", i+1), "nickname": fmt.Sprintf("演示员工%02d", i+1), "password": hex.EncodeToString(secret), "email": fmt.Sprintf("demo.user%02d@example.invalid", i+1), "description": "演示账号：随机不可共享密码，导入后停用，无真实授权", "home_path": "/workspace", "department_id": chosenID(depts, i), "position_ids": []any{chosenID(positions, i)}, "role_ids": []any{chosenID(roles, i)}})
	}
	users, err := run.Ensure(rpc, "core.users", "getUserList", "createUser", "username", Row{}, rows)
	if err != nil {
		return err
	}
	if run.Apply {
		for _, user := range users {
			if number(user["status"]) != 2 {
				if _, err = rpc.Call("updateUser", Row{"id": user["id"], "status": 2}); err != nil {
					return err
				}
			}
		}
	}
	dictionaryNames := []string{"机房园区", "资产用途", "设备品牌", "云资源分类", "应用分类", "项目归属", "采购渠道", "维保厂商", "服务等级", "网络区域", "数据分级", "设备型号", "操作系统", "数据库产品", "业务系统", "服务组件", "交付方式", "供应商类型", "合同类型", "费用中心", "备件类别", "存储类型", "告警分类", "问题分类", "工单分类", "巡检项目", "发布方式", "资产标签", "管理区域", "维护窗口"}
	rows = []Row{}
	for i := 0; i < run.Count; i++ {
		rows = append(rows, Row{"title": demoPrefix + dictionaryNames[i%len(dictionaryNames)], "name": fmt.Sprintf("newbee_demo_dict_%02d", i+1), "status": 1, "desc": "用于平台功能演示的字典，不替换系统字典"})
	}
	dicts, err := run.Ensure(rpc, "core.dictionaries", "getDictionaryList", "createDictionary", "name", Row{}, rows)
	if err != nil {
		return err
	}
	for i, dict := range dicts {
		details := []Row{}
		for j := 0; j < run.Count; j++ {
			details = append(details, Row{"dictionary_id": dict["id"], "title": fmt.Sprintf("%s%02d", dictionaryNames[i%len(dictionaryNames)], j+1), "key": fmt.Sprintf("demo_%02d_%02d", i+1, j+1), "value": fmt.Sprintf("demo-value-%02d", j+1), "status": 1, "sort": j + 1, "is_default": 0, "list_class": []string{"primary", "success", "warning", "info"}[j%4]})
		}
		if _, err = run.Ensure(rpc, fmt.Sprintf("core.dictionary_details.%02d", i+1), "getDictionaryDetailList", "createDictionaryDetail", "value", Row{"dictionary_id": dict["id"]}, details); err != nil {
			return err
		}
	}
	rows = []Row{}
	for i := 0; i < run.Count; i++ {
		rows = append(rows, Row{"name": fmt.Sprintf("%s资产演示参数%02d", demoPrefix, i+1), "key": fmt.Sprintf("demo.asset.config.%02d", i+1), "value": fmt.Sprintf("demo-value-%02d", i+1), "category": "演示数据", "state": false, "sort": i + 100, "remark": "独立演示配置，停用且不覆盖现有系统参数"})
	}
	if _, err = run.Ensure(rpc, "core.configuration", "getConfigurationList", "createConfiguration", "key", Row{}, rows); err != nil {
		return err
	}
	rows = []Row{}
	for i := 0; i < run.Count; i++ {
		rows = append(rows, Row{"name": fmt.Sprintf("%s组织%02d", demoPrefix, i+1), "code": fmt.Sprintf("newbee-demo-tenant-%02d", i+1), "status": 2, "description": "停用的演示租户，不初始化账号或业务数据", "expired_at": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), "config": `{"demo":true,"max_users":0,"features":[]}`})
	}
	if _, err = run.Ensure(rpc, "core.tenants", "getTenantList", "createTenant", "code", Row{}, rows); err != nil {
		return err
	}
	rows = []Row{}
	for i := 0; i < run.Count; i++ {
		rows = append(rows, Row{"name": fmt.Sprintf("newbee-demo-oauth-%02d", i+1), "display_name": fmt.Sprintf("%s企业认证%02d", demoPrefix, i+1), "type": fmt.Sprintf("demo%02d", i+1), "provider_type": "oauth2", "client_id": fmt.Sprintf("demo-client-%02d", i+1), "redirect_url": "https://example.invalid/oauth/callback", "scopes": "profile email", "auth_url": "https://example.invalid/oauth/authorize", "token_url": "https://example.invalid/oauth/token", "info_url": "https://example.invalid/oauth/userinfo", "auth_style": 0, "enabled": false, "status": 2, "tenant_id": 1, "sort": i + 100, "remark": "演示认证提供商；停用，无有效凭据", "extra_config": `{"demo":true}`, "support_pkce": true, "cache_ttl": 300, "success_count": i + 1, "failure_count": i % 4})
	}
	if _, err = run.Ensure(rpc, "core.oauth_providers", "getOauthProviderList", "createOauthProvider", "name", Row{}, rows); err != nil {
		return err
	}
	if len(users) > 0 {
		rows = []Row{}
		for i := 0; i < run.Count; i++ {
			u := users[i%len(users)]
			rows = append(rows, Row{"uuid": u["id"], "username": u["username"], "token": fmt.Sprintf("invalid-demo-token-%02d-not-a-jwt", i+1), "source": "DEMO", "status": 2, "expired_at": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), "tenant_id": 1})
		}
		if _, err = run.Ensure(rpc, "core.revoked_tokens", "getTokenList", "createToken", "token", Row{}, rows); err != nil {
			return err
		}
		rows = []Row{}
		for i := 0; i < run.Count; i++ {
			u := users[i%len(users)]
			rows = append(rows, Row{"tenant_id": "1", "user_id": u["id"], "user_name": u["nickname"], "operation_type": []string{"CREATE", "READ", "UPDATE", "DELETE"}[i%4], "resource_type": "DEMO_ASSET", "resource_id": fmt.Sprintf("newbee-demo-audit-%02d", i+1), "request_method": []string{"POST", "GET", "PUT", "DELETE"}[i%4], "request_path": fmt.Sprintf("/demo/assets/%02d", i+1), "request_data": `{"demo":true}`, "response_status": 200, "response_data": `{"code":0,"demo":true}`, "ip_address": fmt.Sprintf("192.0.2.%d", i+1), "user_agent": "Newbee Demo Seeder", "duration_ms": 20 + i, "metadata": `{"source":"demo_seed","synthetic":true}`})
		}
		if _, err = run.Ensure(rpc, "core.audit_logs", "getAuditLogList", "createAuditLog", "resource_id", Row{}, rows); err != nil {
			return err
		}
	}
	for _, item := range []struct{ entity, method string }{{"core.menus", "getMenuList"}, {"core.api_catalog", "getApiList"}} {
		_, total, err := rpc.List(item.method, Row{})
		if err != nil {
			return err
		}
		if total < 20 {
			return fmt.Errorf("%s has only %d rows", item.entity, total)
		}
		run.Results = append(run.Results, EntityResult{Entity: item.entity, Before: total, After: total, Notes: "现有真实功能目录已超过一页，保留原记录"})
	}
	return nil
}
