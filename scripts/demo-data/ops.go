package main

import (
	"fmt"
	"time"
)

func init() { seeders["ops"] = seedOps }
func seedOps(run *SeedRun) error {
	rpc, err := run.OpenRPC("127.0.0.1:9600", "ops.Ops")
	if err != nil {
		return err
	}
	defer rpc.Close()
	ensure := func(entity, key string, rows []Row) ([]Row, error) {
		return run.Ensure(rpc, "ops."+entity, "get"+entity+"List", "create"+entity, key, Row{}, rows)
	}
	agents, proxies, agroups, pgroups, categories := []Row{}, []Row{}, []Row{}, []Row{}, []Row{}
	regions := []string{"cn-beijing", "cn-shanghai", "cn-guangzhou"}
	for i := 1; i <= run.Count; i++ {
		region := regions[(i-1)%3]
		name := fmt.Sprintf("演示-%s-%02d", region, i)
		agents = append(agents, Row{"name": name + "采集节点", "agent_id": fmt.Sprintf("demo-agent-%02d", i), "host": fmt.Sprintf("192.0.2.%d", i), "port": 9002, "status": 2, "agent_status": "offline", "api_key": "", "description": "演示离线节点，不连接真实设备", "region": region, "heartbeat_interval": 60, "max_concurrent_tasks": 1, "capabilities": "[\"ssh\"]", "tags": "[\"演示\"]", "version": "demo-1.0"})
		proxies = append(proxies, Row{"name": name + "代理", "proxy_id": fmt.Sprintf("demo-proxy-%02d", i), "ip": fmt.Sprintf("198.51.100.%d", i), "port": 9001, "status": 2, "proxy_status": "offline", "region": region, "zone": "demo-zone", "weight": 1, "priority": 0, "max_sessions": 1, "capabilities": "[\"ssh\"]", "tags": "[\"演示\"]", "version": "demo-1.0"})
		group := Row{"name": name + "分组", "description": "演示禁用分组", "status": 2, "selection_strategy": "round_robin", "health_check_interval": 60, "auto_failover": false, "max_retry_count": 0}
		agroups = append(agroups, group)
		pgroups = append(pgroups, group)
		categories = append(categories, Row{"name": fmt.Sprintf("演示-资产运维分类-%02d", i), "code": fmt.Sprintf("demo-category-%02d", i), "description": "演示脚本分类", "status": 2, "sort_order": i})
	}
	for _, x := range []struct {
		e, k string
		r    []Row
	}{{"Agent", "agent_id", agents}, {"Proxy", "proxy_id", proxies}, {"AgentGroup", "name", agroups}, {"ProxyGroup", "name", pgroups}} {
		if _, err = ensure(x.e, x.k, x.r); err != nil {
			return err
		}
	}
	cats, err := ensure("ScriptCategory", "code", categories)
	if err != nil {
		return err
	}
	scripts := []Row{}
	for i := 1; i <= run.Count; i++ {
		row := Row{"name": fmt.Sprintf("演示-资产巡检脚本-%02d", i), "code": fmt.Sprintf("demo-inspection-%02d", i), "description": "演示占位脚本，禁用且不可调度", "status": 2, "script_type": "shell", "executor": "agent", "content": "# Demonstration only; deliberately exits without accessing any target.\nexit 0\n", "version": "1.0.0", "is_latest": true, "schedulable": false, "require_confirmation": true, "risk_level": "low", "parameters": "{}", "tags": "[\"演示\"]"}
		if len(cats) > 0 {
			row["category_id"] = cats[(i-1)%len(cats)]["id"]
		}
		scripts = append(scripts, row)
	}
	if _, err = ensure("Script", "code", scripts); err != nil {
		return err
	}
	// These RPCs persist historical records only. Agent executor avoids proxy selection/dispatch.
	cmdb, err := run.OpenRPC("127.0.0.1:9200", "cmdb.Cmdb")
	if err != nil {
		return err
	}
	defer cmdb.Close()
	cis, _, err := cmdb.List("getCisList", Row{})
	if err != nil {
		return err
	}
	demoCIs := []Row{}
	for _, ci := range cis {
		entries, _ := ci["metadata"].([]any)
		for _, entry := range entries {
			marker, ok := entry.(map[string]any)
			if ok && textValue(marker["key"]) == "demo" && textValue(marker["value"]) == "newbee-demo-v1" {
				demoCIs = append(demoCIs, ci)
				break
			}
		}
	}
	cis = demoCIs
	profiles, sessions, tasks := []Row{}, []Row{}, []Row{}
	for i := 1; i <= run.Count; i++ {
		ci := fmt.Sprintf("demo-ci-%02d", i)
		if len(cis) > 0 {
			ci = textValue(cis[(i-1)%len(cis)]["id"])
		}
		profiles = append(profiles, Row{"ci_id": ci, "status": 2, "capabilities": Row{"list": []string{"ssh"}}, "ports": Row{"ssh": 22}, "credential_ref": fmt.Sprintf("demo://credential/%02d", i), "tags": Row{"demo": "true"}})
		stamp := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute).Unix()
		sessions = append(sessions, Row{"session_id": fmt.Sprintf("demo-session-%02d", i), "user_id": "demo-operator", "ci_id": ci, "protocol": "ssh", "proxy_id": fmt.Sprintf("demo-proxy-%02d", i), "status_str": "closed", "expires_at": stamp, "closed_at": stamp, "tags": Row{"demo": "true"}})
		tasks = append(tasks, Row{"task_id": fmt.Sprintf("demo-task-%02d", i), "name": fmt.Sprintf("演示-资产巡检记录-%02d", i), "creator_id": "demo-operator", "ci_ids": Row{"list": []string{ci}}, "executor": "agent", "status": 2, "status_str": "success", "command_content": "# demo history only", "command_timeout": 30, "result_output": "演示历史：资产配置检查完成，无实际执行", "start_time": stamp - 10, "end_time": stamp, "exit_code": 0, "tags": Row{"demo": "true"}})
	}
	if run.Apply && len(cis) < run.Count {
		return fmt.Errorf("ops requires %d existing CI rows before access profiles, have %d", run.Count, len(cis))
	}
	for _, x := range []struct {
		e, k string
		r    []Row
	}{{"AccessProfile", "ci_id", profiles}, {"Session", "session_id", sessions}, {"Task", "task_id", tasks}} {
		if _, err = ensure(x.e, x.k, x.r); err != nil {
			return err
		}
	}
	metrics := []Row{}
	for i := 1; i <= run.Count; i++ {
		metrics = append(metrics, Row{"proxy_id": "demo-proxy-01", "timestamp": time.Date(2026, 10, 5, 14, i, 0, 0, time.UTC).UnixMilli(), "cpu_usage": 10 + i%20, "memory_usage": 30 + i%25, "proxy_status": "offline", "active_sessions": 0, "request_count_delta": 10, "success_count_delta": 10})
	}
	if _, err = run.Ensure(rpc, "ops.ProxyMetrics", "getProxyMetricsList", "createProxyMetrics", "timestamp", Row{"proxy_id": "demo-proxy-01"}, metrics); err != nil {
		return err
	}

	return nil
}
