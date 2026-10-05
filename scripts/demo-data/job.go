package main

import "fmt"

func init() { seeders["job"] = seedJob }

func seedJob(run *SeedRun) error {
	rpc, err := run.OpenRPC("127.0.0.1:9105", "job.Job")
	if err != nil {
		return err
	}
	defer rpc.Close()
	rows := []Row{}
	names := []string{"资产清单日汇总", "配置一致性核验", "设备保修到期提醒", "机房容量月报", "资产变更审计"}
	for i := 1; i <= run.Count; i++ {
		rows = append(rows, Row{"name": fmt.Sprintf("演示-%s-%02d", names[(i-1)%len(names)], i), "task_group": "newbee-demo-disabled", "status": 2, "cron_expression": "0 0 3 * * *", "pattern": fmt.Sprintf("demo.disabled.%02d", i), "payload": `{"demo":true,"disabled":true}`})
	}
	_, err = run.Ensure(rpc, "job.tasks", "getTaskList", "createTask", "name", Row{}, rows)
	return err
}
