package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/coder-lulu/newbee-common/v2/middleware/keys"
	"google.golang.org/grpc/metadata"
)

func init() { seeders["isolation"] = verifyDemoIsolation }

// This read-only check scopes the existing superadmin's diagnostic requests to
// the restored second tenant. Global catalogs and metrics are deliberately excluded.
func verifyDemoIsolation(run *SeedRun) error {
	md, ok := metadata.FromOutgoingContext(run.Context)
	if !ok || !strings.Contains(strings.Join(md.Get(keys.RoleCodesKey.String()), ","), "superadmin") {
		return fmt.Errorf("isolation check requires the real local superadmin")
	}
	md = md.Copy()
	md.Set(keys.TenantIDKey.String(), "2")
	reader := &SeedRun{Context: metadata.NewOutgoingContext(run.Context, md)}
	checks := []struct{ entity, address, service, method, key, prefix string }{
		{"core.users", "127.0.0.1:9100", "core.Core", "getUserList", "username", "newbee_demo_user_"},
		{"core.departments", "127.0.0.1:9100", "core.Core", "getDepartmentList", "name", demoPrefix},
		{"cmdb.cis", "127.0.0.1:9200", "cmdb.Cmdb", "getCisList", "metadata.demo", "newbee-demo-v1"},
		{"io.data_targets", "127.0.0.1:9500", "io.Io", "getDataTargetList", "target_code", "newbee-demo-target-"},
		{"ops.proxies", "127.0.0.1:9600", "ops.Ops", "getProxyList", "proxy_id", "demo-proxy-"},
	}
	results := []Row{}
	for _, check := range checks {
		rpc, err := reader.OpenRPC(check.address, check.service)
		if err != nil {
			return err
		}
		rows, total, err := rpc.List(check.method, Row{})
		rpc.Close()
		if err != nil {
			return err
		}
		demo := 0
		for _, row := range rows {
			if strings.HasPrefix(textValue(rowValue(row, check.key)), check.prefix) {
				demo++
			}
		}
		results = append(results, Row{"entity": check.entity, "tenant_id": 2, "total": total, "demo_records": demo, "passed": demo == 0})
		if demo != 0 {
			return fmt.Errorf("tenant 2 can see %d tenant 1 demo rows in %s", demo, check.entity)
		}
	}
	body, _ := json.MarshalIndent(Row{"passed": true, "checks": results}, "", "  ")
	if err := os.WriteFile("logs/demo-data/isolation-verification.json", body, 0600); err != nil {
		return err
	}
	fmt.Printf("tenant isolation: %d scoped business lists passed\n", len(results))
	return nil
}
