package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"entgo.io/ent/dialect/sql/schema"

	"github.com/coder-lulu/newbee-common/v2/middleware/keys"
	"github.com/coder-lulu/newbee-common/v2/orm/ent/hooks"
	ioent "github.com/coder-lulu/newbee-io-rpc/ent"
	"github.com/coder-lulu/newbee-io-rpc/ent/cichangehistory"
	"github.com/coder-lulu/newbee-io-rpc/ent/cilifecyclestate"
	_ "github.com/coder-lulu/newbee-io-rpc/ent/runtime"
	jobent "github.com/coder-lulu/newbee-job/ent"
	_ "github.com/coder-lulu/newbee-job/ent/runtime"
	"github.com/coder-lulu/newbee-job/ent/task"
	"github.com/coder-lulu/newbee-job/ent/tasklog"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/metadata"
)

func init() {
	seeders["io-extra"] = seedIOExtra
	seeders["job-history"] = seedJobHistory
	seeders["io-audit-schema"] = seedIOAuditSchema
}

func seedIOAuditSchema(run *SeedRun) error {
	ctx, dsn, err := demoEntConnection(run)
	if err != nil {
		return err
	}
	if !run.Apply {
		fmt.Println("io-audit-schema: plan only ci_change_histories, ci_lifecycle_states; no other tables")
		run.Results = append(run.Results, EntityResult{Entity: "io.audit_schema", Notes: "仅两张审计表；未开启删除表/列/索引"})
		return nil
	}
	client, err := ioent.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("IO local database connection failed")
	}
	defer client.Close()
	if err = setupDemoEnt(client); err != nil {
		return err
	}
	err = client.Schema.Create(ctx, schema.WithHooks(func(next schema.Creator) schema.Creator {
		return schema.CreateFunc(func(ctx context.Context, tables ...*schema.Table) error {
			selected := []*schema.Table{}
			for _, table := range tables {
				if table.Name == "ci_change_histories" || table.Name == "ci_lifecycle_states" {
					selected = append(selected, table)
				}
			}
			if len(selected) != 2 {
				return fmt.Errorf("expected exactly two IO audit schema definitions")
			}
			return next.Create(ctx, selected...)
		})
	}))
	if err != nil {
		return err
	}
	run.Results = append(run.Results, EntityResult{Entity: "io.audit_schema", After: 2, Notes: "仅两张IO审计表；无删除选项"})
	return nil
}

// QuickSetup initializes global hook configuration once; each additional Ent
// client must separately register the configured hooks.
var demoEntHooksInitialized bool

func setupDemoEnt(client any) error {
	if demoEntHooksInitialized {
		return hooks.RegisterAllHooks(client)
	}
	if err := hooks.QuickSetup(client); err != nil {
		return err
	}
	demoEntHooksInitialized = true
	return nil
}

func demoEntConnection(run *SeedRun) (context.Context, string, error) {
	data, err := os.ReadFile("logs/local-run/private-runtime.json")
	if err != nil {
		return nil, "", fmt.Errorf("local runtime file unavailable")
	}
	var config struct {
		Database string `json:"database"`
		User     string `json:"mysql_user"`
		Password string `json:"mysql_password"`
	}
	if json.Unmarshal(data, &config) != nil || config.Database != "newbee" || config.User == "" || config.Password == "" {
		return nil, "", fmt.Errorf("local newbee database configuration required")
	}
	md, ok := metadata.FromOutgoingContext(run.Context)
	if !ok || len(md.Get(keys.TenantIDKey.String())) != 1 || md.Get(keys.TenantIDKey.String())[0] != "1" || len(md.Get(keys.UserIDKey.String())) == 0 {
		return nil, "", fmt.Errorf("authenticated tenant 1 context required")
	}
	ctx := metadata.NewIncomingContext(run.Context, md.Copy())
	cm := keys.NewContextManager()
	ctx = cm.SetTenantID(ctx, "1")

	ctx = cm.SetUserID(ctx, md.Get(keys.UserIDKey.String())[0])
	if v := md.Get(keys.DeptIDKey.String()); len(v) > 0 {
		ctx = cm.SetDeptID(ctx, v[0])
	}
	if v := md.Get(keys.RoleCodesKey.String()); len(v) > 0 {
		ctx = cm.SetRoleCodes(ctx, v[0])
	}
	if v := md.Get(keys.DataScopeKey.String()); len(v) > 0 {
		ctx = cm.SetDataScope(ctx, v[0])
	}
	cfg := mysql.NewConfig()
	cfg.User = config.User
	cfg.Passwd = config.Password
	cfg.Net = "tcp"
	cfg.Addr = "127.0.0.1:3306"
	cfg.DBName = "newbee"
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	return ctx, cfg.FormatDSN(), nil
}

func seedIOExtra(run *SeedRun) error {
	rpc, err := run.OpenRPC("127.0.0.1:9500", "io.Io")
	if err != nil {
		return err
	}
	defer rpc.Close()
	desired := []Row{}
	for n := 1; n <= run.Count; n++ {
		desired = append(desired, Row{"provider_id": fmt.Sprintf("demo.provider.schema.%02d", n), "provider_name": fmt.Sprintf("演示-资产文件Schema%02d", n), "category": "file", "status": 2, "execution_mode": "direct", "is_active": false, "is_builtin": false, "version": "1.0-demo", "description": "停用的演示字段定义，不执行发现", "parameter_schema": base64.StdEncoding.EncodeToString([]byte(`[{"name":"file_path","label":"演示文件路径","type":"string","required":false}]`)), "field_schema": base64.StdEncoding.EncodeToString([]byte(`[{"name":"asset_code","label":"资产编码","type":"string"},{"name":"asset_name","label":"资产名称","type":"string"},{"name":"ip","label":"管理IP","type":"string"}]`))})
	}
	if _, err = run.Ensure(rpc, "io.provider_schemas", "getDiscoveryProviderSchemaList", "createDiscoveryProviderSchema", "provider_id", Row{}, desired); err != nil {
		return err
	}
	cmdb, err := run.OpenRPC("127.0.0.1:9200", "cmdb.Cmdb")
	if err != nil {
		return err
	}
	defer cmdb.Close()
	all, _, err := cmdb.List("getCisList", Row{})
	if err != nil {
		return err
	}
	cis := []Row{}
	for _, ci := range all {
		if textValue(rowValue(ci, "metadata.demo")) == "newbee-demo-v1" {
			cis = append(cis, ci)
		}
	}
	if len(cis) < run.Count {
		if run.Apply {
			return fmt.Errorf("io-extra requires %d demo CIs", run.Count)
		}
		run.Results = append(run.Results, EntityResult{Entity: "io.ci_audit.pending_cis", Added: run.Count * 2, Notes: "等待CMDB演示CI，不创建无关联历史"})
		return nil
	}
	ctx, dsn, err := demoEntConnection(run)
	if err != nil {
		return err
	}
	client, err := ioent.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("IO local database connection failed")
	}
	defer client.Close()
	if err = setupDemoEnt(client); err != nil {
		return err
	}
	beforeHistory, err := client.CiChangeHistory.Query().Where(cichangehistory.TenantIDEQ(1)).Count(ctx)
	if err != nil {
		return err
	}
	beforeState, err := client.CiLifecycleState.Query().Where(cilifecyclestate.TenantIDEQ(1)).Count(ctx)
	if err != nil {
		return err
	}
	addedHistory, addedState := 0, 0
	for n := 1; n <= run.Count; n++ {
		ci := cis[n-1]
		ciID, typeID := uint64(number(ci["id"])), uint64(number(ci["type_id"]))
		op := fmt.Sprintf("demo.change.%03d", n)
		state := fmt.Sprintf("demo.lifecycle.%03d", n)
		started := time.Date(2026, 1, 10, 9, n, 0, 0, time.UTC)
		finished := started.Add(5 * time.Minute)
		exists, e := client.CiChangeHistory.Query().Where(cichangehistory.TenantIDEQ(1), cichangehistory.OperationIDEQ(op)).Exist(ctx)
		if e != nil {
			return e
		}
		if !exists {
			addedHistory++
			if run.Apply {
				_, e = client.CiChangeHistory.Create().SetOperationID(op).SetCiID(ciID).SetCiTypeID(typeID).SetOperationType("update").SetOperationName("演示资产信息维护").SetOperatorName("演示资产管理员").SetChangeReason("演示-资产台账定期核验").SetSource("manual").SetSourceDetail("newbee-demo-v1").SetChangedFields(`[{"field":"remark","old_value":"待核验","new_value":"演示核验完成"}]`).SetStatus("success").SetNeedsApproval(false).SetCanRollback(false).SetAffectedCount(1).SetCreatedAt(started).SetUpdatedAt(finished).Save(ctx)
				if e != nil {
					return e
				}
			}
		}
		exists, e = client.CiLifecycleState.Query().Where(cilifecyclestate.TenantIDEQ(1), cilifecyclestate.StateIDEQ(state)).Exist(ctx)
		if e != nil {
			return e
		}
		if !exists {
			addedState++
			if run.Apply {
				_, e = client.CiLifecycleState.Create().SetStateID(state).SetCiID(ciID).SetCiTypeID(typeID).SetStateName("演示资产核验完成").SetStateType("completed").SetPreviousState("executed").SetEnteredAt(started).SetExitedAt(finished).SetDurationSeconds(300).SetTriggerType("manual").SetTriggeredByName("演示资产管理员").SetIsCurrent(true).SetIsFinal(true).SetCanRetry(false).SetOperationID(op).SetComment("newbee-demo-v1; 历史终态，不执行任务").SetCreatedAt(started).SetUpdatedAt(finished).Save(ctx)
				if e != nil {
					return e
				}
			}
		}
	}
	historyCount, err := client.CiChangeHistory.Query().Where(cichangehistory.TenantIDEQ(1), cichangehistory.OperationIDHasPrefix("demo.change.")).Count(ctx)
	if err != nil {
		return err
	}
	stateCount, err := client.CiLifecycleState.Query().Where(cilifecyclestate.TenantIDEQ(1), cilifecyclestate.StateIDHasPrefix("demo.lifecycle.")).Count(ctx)
	if err != nil {
		return err
	}
	if run.Apply && (historyCount < run.Count || stateCount < run.Count) {
		return fmt.Errorf("IO audit read-back count mismatch")
	}
	run.Results = append(run.Results, EntityResult{Entity: "io.ci_change_history", Before: beforeHistory, Added: addedHistory, After: beforeHistory + conditionalAdded(run.Apply, addedHistory), Demo: historyCount}, EntityResult{Entity: "io.ci_lifecycle_state", Before: beforeState, Added: addedState, After: beforeState + conditionalAdded(run.Apply, addedState), Demo: stateCount})
	return nil
}

func conditionalAdded(apply bool, n int) int {
	if apply {
		return n
	}
	return 0
}

func seedJobHistory(run *SeedRun) error {
	ctx, dsn, err := demoEntConnection(run)
	if err != nil {
		return err
	}
	client, err := jobent.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("Job local database connection failed")
	}
	defer client.Close()
	if err = setupDemoEnt(client); err != nil {
		return err
	}
	// Job's existing schema defines these two tables as global (no TenantMixin).
	// Keep the unified hooks registered and declare only their exact schema scope.
	hooks.AddExcludedTable("sys_tasks")
	hooks.AddExcludedTable("sys_task_logs")
	if config := hooks.GlobalHookManager.GetConfig(hooks.FieldTypeTenant); config != nil {
		previous := append([]string(nil), config.ExcludedEntities...)
		defer func() { config.ExcludedEntities = previous }()
		config.ExcludedEntities = append(config.ExcludedEntities, "Task", "TaskLog")
	}
	tasks, err := client.Task.Query().Where(task.TaskGroupEQ("newbee-demo-disabled"), task.PatternHasPrefix("demo.disabled."), task.StatusEQ(2)).All(ctx)
	if err != nil {
		return err
	}
	if len(tasks) < run.Count {
		if run.Apply {
			return fmt.Errorf("job-history requires %d disabled demo tasks", run.Count)
		}
		run.Results = append(run.Results, EntityResult{Entity: "job.history.pending_tasks", Added: run.Count * run.Count, Notes: "等待停用演示任务；TaskLog无TenantMixin，不伪造tenant字段"})
		return nil
	}
	for _, item := range tasks {
		q := client.TaskLog.Query().Where(tasklog.HasTasksWith(task.IDEQ(item.ID)))
		before, e := q.Clone().Count(ctx)
		if e != nil {
			return e
		}
		added := 0
		for n := 1; n <= run.Count; n++ {
			started := time.Date(2026, 1, n, 3, 0, 0, 0, time.UTC)
			exists, e := q.Clone().Where(tasklog.StartedAtEQ(started)).Exist(ctx)
			if e != nil {
				return e
			}
			if !exists {
				added++
				if run.Apply {
					if _, e = client.TaskLog.Create().SetTasksID(item.ID).SetStartedAt(started).SetFinishedAt(started.Add(5 * time.Second)).SetResult(1).Save(ctx); e != nil {
						return e
					}
				}
			}
		}
		after, e := q.Clone().Count(ctx)
		if e != nil {
			return e
		}
		verified := 0
		for n := 1; n <= run.Count; n++ {
			exists, e := q.Clone().Where(tasklog.StartedAtEQ(time.Date(2026, 1, n, 3, 0, 0, 0, time.UTC))).Exist(ctx)
			if e != nil {
				return e
			}
			if exists {
				verified++
			}
		}
		if run.Apply && verified != run.Count {
			return fmt.Errorf("job task %d history read-back mismatch", item.ID)
		}
		run.Results = append(run.Results, EntityResult{Entity: fmt.Sprintf("job.history.task_%d", item.ID), Before: before, Added: added, After: after, Demo: verified, Notes: "仅停用演示任务的历史日志；实体没有TenantMixin"})
	}
	return nil
}
