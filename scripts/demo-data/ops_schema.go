package main

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql/schema"
	opsent "github.com/coder-lulu/newbee-ops-rpc/ent"
	_ "github.com/coder-lulu/newbee-ops-rpc/ent/runtime"
)

func init() { seeders["ops-schema"] = seedOpsSchema }

// The restored sessions table predates nullable worker bindings. Tasks and
// ops_worker_metrics already contain every column used by the current readers.
// Keep this migration limited to the one verified incomplete table.
func seedOpsSchema(run *SeedRun) error {
	if !run.Apply {
		fmt.Println("ops-schema: plan only sessions; add nullable worker_id, worker_ip, worker_port; no drop operations")
		run.Results = append(run.Results, EntityResult{Entity: "ops.session_schema", Notes: "计划仅补sessions可空worker_id/worker_ip/worker_port；不迁移tasks或ops_worker_metrics，不删除列或索引"})
		return nil
	}
	ctx, dsn, err := demoEntConnection(run)
	if err != nil {
		return err
	}
	client, err := opsent.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("Ops local database connection failed")
	}
	defer client.Close()
	if err = setupDemoEnt(client); err != nil {
		return err
	}
	err = client.Schema.Create(ctx,
		schema.WithDropColumn(false),
		schema.WithDropIndex(false),
		schema.WithHooks(func(next schema.Creator) schema.Creator {
			return schema.CreateFunc(func(ctx context.Context, tables ...*schema.Table) error {
				selected := []*schema.Table{}
				for _, table := range tables {
					if table.Name == "sessions" {
						selected = append(selected, table)
					}
				}
				if len(selected) != 1 {
					return fmt.Errorf("expected exactly one Ops sessions schema definition")
				}
				return next.Create(ctx, selected...)
			})
		}),
	)
	if err != nil {
		return err
	}
	run.Results = append(run.Results, EntityResult{Entity: "ops.session_schema", After: 1, Notes: "仅迁移sessions；禁用删除列和索引；tasks/ops_worker_metrics保持不变"})
	return nil
}
