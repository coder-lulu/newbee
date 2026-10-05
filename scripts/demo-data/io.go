package main

import (
	"fmt"
	"time"
)

func init() { seeders["io"] = seedIO }

func seedIO(run *SeedRun) error {
	rpc, err := run.OpenRPC("127.0.0.1:9500", "io.Io")
	if err != nil {
		return err
	}
	defer rpc.Close()
	metadata := `{"demo":true,"source":"newbee-demo","execution":"disabled"}`
	domains := []string{"服务器", "交换机", "数据库", "虚拟机", "应用服务", "存储设备"}
	rows := func(makeRow func(int, string) Row) []Row {
		result := make([]Row, 0, run.Count)
		for i := 0; i < run.Count; i++ {
			result = append(result, makeRow(i, domains[i%len(domains)]))
		}
		return result
	}
	ensure := func(entity, key string, desired []Row) ([]Row, error) {
		return run.Ensure(rpc, "io."+entity, "Get"+entity+"List", "Create"+entity, key, Row{}, desired)
	}
	_, err = ensure("DiscoveryTemplate", "template_code", rows(func(i int, d string) Row {
		return Row{"template_name": fmt.Sprintf("演示-%s发现模板-%02d", d, i+1), "template_code": fmt.Sprintf("newbee-demo-template-%02d", i+1), "description": "仅用于演示资产发现配置，不连接真实资源", "template_type": "standard", "version": "1.0.0", "discovery_config": `{"demo":true,"enabled":false}`, "field_mapping_templates": `[{"source":"hostname","target":"name"}]`, "validation_rules": `{"required":["hostname"]}`, "is_public": false, "is_system": false, "tags": "演示,资产管理", "metadata": metadata, "status": 1}
	}))
	if err != nil {
		return err
	}
	targets, err := ensure("DataTarget", "target_code", rows(func(i int, d string) Row {
		return Row{"target_name": fmt.Sprintf("演示-%s归档目标-%02d", d, i+1), "target_code": fmt.Sprintf("newbee-demo-target-%02d", i+1), "target_type": []string{"file", "api", "database"}[i%3], "target_system": "新蜂演示资产库", "description": "演示目标，已禁用", "connection_config": `{"demo":true,"endpoint":"https://example.invalid/assets"}`, "is_active": false, "metadata": metadata, "status": 1}
	}))
	if err != nil {
		return err
	}
	pools, err := ensure("DiscoveryPool", "name", rows(func(i int, d string) Row {
		return Row{"name": fmt.Sprintf("演示-%s发现池-%02d", d, i+1), "description": "已停用的演示发现池", "discovery_type": []string{"file", "api", "builtin"}[i%3], "pool_status": "inactive", "approval_status": "pending", "discovery_config": `{"demo":true,"enabled":false}`, "schedule": "", "batch_size": 100, "concurrent_limit": 1, "max_retry": 0, "retry_interval": 60, "field_mapping": `{"hostname":"name","ip":"ip_address"}`, "total_runs": 10 + i, "success_runs": 10 + i, "failed_runs": 0, "metadata": metadata, "status": 1}
	}))
	if err != nil {
		return err
	}
	relation := func(data []Row, i int) any {
		if len(data) == 0 {
			return nil
		}
		return rowID(data[i%len(data)])
	}
	inputs, err := ensure("InputTask", "task_name", rows(func(i int, d string) Row {
		row := Row{"task_name": fmt.Sprintf("演示-%s资产导入-%02d", d, i+1), "task_type": "manual", "input_source": []string{"file", "api", "database"}[i%3], "source_config": `{"demo":true,"enabled":false}`, "task_status": "completed", "total_records": 100 + i, "processed_records": 100 + i, "success_records": 100 + i, "failed_records": 0, "started_at": time.Now().Add(-time.Duration(i+2) * time.Hour).UnixMilli(), "completed_at": time.Now().Add(-time.Duration(i+1) * time.Hour).UnixMilli(), "metadata": metadata, "status": 1}
		if id := relation(pools, i); id != nil {
			row["discovery_pool_id"] = id
		}
		return row
	}))
	if err != nil {
		return err
	}
	outputs, err := ensure("OutputTask", "task_name", rows(func(i int, d string) Row {
		row := Row{"task_name": fmt.Sprintf("演示-%s资产导出-%02d", d, i+1), "task_type": "manual", "output_target": []string{"file", "api", "database"}[i%3], "target_config": `{"demo":true,"enabled":false}`, "task_status": "completed", "total_records": 50 + i, "processed_records": 50 + i, "success_records": 50 + i, "failed_records": 0, "started_at": time.Now().Add(-2 * time.Hour).UnixMilli(), "completed_at": time.Now().Add(-time.Hour).UnixMilli(), "metadata": metadata, "status": 1}
		if id := relation(targets, i); id != nil {
			row["data_target_id"] = id
		}
		return row
	}))
	if err != nil {
		return err
	}
	mappings, err := ensure("FieldMapping", "mapping_name", rows(func(i int, d string) Row {
		row := Row{"mapping_name": fmt.Sprintf("演示-%s字段映射-%02d", d, i+1), "description": "资产字段标准化演示", "mapping_type": "input", "is_active": false, "source_field": []string{"hostname", "ip", "serial_number"}[i%3], "target_field": []string{"name", "ip_address", "asset_code"}[i%3], "source_data_type": "string", "target_data_type": "string", "transform_type": "direct", "transform_config": "{}", "allow_null": false, "is_required": true, "priority": i + 1, "sort_order": i + 1, "metadata": metadata, "status": 1}
		if id := relation(inputs, i); id != nil {
			row["input_task_id"] = id
		}
		if id := relation(outputs, i); id != nil {
			row["output_task_id"] = id
		}
		return row
	}))
	if err != nil {
		return err
	}
	_, err = ensure("TaskLog", "log_message", rows(func(i int, d string) Row {
		taskID := relation(inputs, i)
		if taskID == nil {
			taskID = 0
		}
		return Row{"task_type": "input", "task_id": taskID, "log_level": []string{"info", "warn", "info"}[i%3], "log_message": fmt.Sprintf("演示-%s资产校验完成-%02d", d, i+1), "log_detail": metadata, "logged_at": time.Now().Add(-time.Duration(i+1) * time.Minute).UnixMilli()}
	}))
	if err != nil {
		return err
	}
	_, err = ensure("MappingLog", "source_value", rows(func(i int, d string) Row {
		mappingID := relation(mappings, i)
		if mappingID == nil {
			mappingID = 0
		}
		return Row{"field_mapping_id": mappingID, "source_value": fmt.Sprintf("newbee-demo-host-%02d", i+1), "target_value": fmt.Sprintf("演示-%s-%02d", d, i+1), "transform_status": "success", "logged_at": time.Now().Add(-time.Duration(i+1) * time.Minute).UnixMilli()}
	}))
	if err != nil {
		return err
	}
	_, err = ensure("WorkerMetrics", "worker_id", rows(func(i int, d string) Row {
		return Row{"worker_id": fmt.Sprintf("newbee-demo-worker-%02d", i+1), "worker_name": fmt.Sprintf("演示-%s采集节点-%02d", d, i+1), "worker_status": "offline", "current_tasks": 0, "total_tasks": 100 + i, "success_tasks": 100 + i, "failed_tasks": 0, "cpu_usage": float64(10 + i), "memory_usage": float64(25 + i), "last_heartbeat": time.Now().Add(-time.Hour).UnixMilli(), "metadata": metadata}
	}))
	if err != nil {
		return err
	}
	_, err = ensure("CronTask", "task_name", rows(func(i int, d string) Row {
		return Row{"task_name": fmt.Sprintf("演示-%s定时采集-%02d", d, i+1), "cron_expression": "0 3 * * *", "input_source": "file", "source_config": `{"demo":true,"enabled":false}`, "enabled": false, "description": "仅用于演示，调度已禁用", "status": 1}
	}))
	if err != nil {
		return err
	}
	_, err = run.Ensure(rpc, "io.Config", "ListConfig", "CreateConfig", "config_key", Row{"keyword": "demo.newbee."}, rows(func(i int, d string) Row {
		return Row{"config_key": fmt.Sprintf("demo.newbee.asset.%02d", i+1), "config_value": fmt.Sprintf("演示-%s资产配置-%02d", d, i+1), "value_type": "string", "category": "demo", "service_name": "newbee-demo", "description": "演示配置，业务代码不读取此命名空间", "is_sensitive": false, "is_readonly": false, "scope": "tenant", "config_group": "newbee-demo"}
	}))
	return err
}
