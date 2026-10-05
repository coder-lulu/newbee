// Demo data uses the existing tenant-aware RPC services. It never runs tasks.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/coder-lulu/newbee-cmdb-rpc/types/cmdb"
	"github.com/coder-lulu/newbee-common/v2/middleware/keys"
	_ "github.com/coder-lulu/newbee-core/rpc/types/core"
	_ "github.com/coder-lulu/newbee-io-rpc/types/io"
	_ "github.com/coder-lulu/newbee-job/types/job"
	_ "github.com/coder-lulu/newbee-ops-rpc/types/ops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Row = map[string]any
type EntityResult struct {
	Entity string `json:"entity"`
	Before int    `json:"before"`
	Added  int    `json:"added"`
	After  int    `json:"after"`
	Demo   int    `json:"demo_records"`
	Notes  string `json:"notes,omitempty"`
}
type SeedRun struct {
	Context context.Context
	Apply   bool
	Count   int
	Results []EntityResult
}

var seeders = map[string]func(*SeedRun) error{}

const demoPrefix = "演示-"

type RPC struct {
	conn    *grpc.ClientConn
	service protoreflect.ServiceDescriptor
	run     *SeedRun
}

func (run *SeedRun) OpenRPC(address, service string) (*RPC, error) {
	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil, err
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("not a service: %s", service)
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &RPC{conn, sd, run}, nil
}
func (r *RPC) Close() { _ = r.conn.Close() }
func (r *RPC) method(name string) (protoreflect.MethodDescriptor, error) {
	for i := 0; i < r.service.Methods().Len(); i++ {
		m := r.service.Methods().Get(i)
		if strings.EqualFold(string(m.Name()), name) {
			return m, nil
		}
	}
	return nil, fmt.Errorf("unknown method %s.%s", r.service.FullName(), name)
}
func (r *RPC) Call(name string, request Row) (Row, error) {
	m, err := r.method(name)
	if err != nil {
		return nil, err
	}
	input := dynamicpb.NewMessage(m.Input())
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if err = protojson.Unmarshal(body, input); err != nil {
		return nil, fmt.Errorf("%s input: %w", name, err)
	}
	output := dynamicpb.NewMessage(m.Output())
	ctx, cancel := context.WithTimeout(r.run.Context, 30*time.Second)
	defer cancel()
	if err = r.conn.Invoke(ctx, "/"+string(r.service.FullName())+"/"+string(m.Name()), input, output); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	encoded, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(output)
	if err != nil {
		return nil, err
	}
	var row Row
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	err = decoder.Decode(&row)
	return row, err
}
func number(v any) int { n, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64); return int(n) }
func textValue(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func rowID(row Row) any { return row["id"] }
func rowValue(row Row, key string) any {
	parts := strings.SplitN(key, ".", 2)
	value := row[parts[0]]
	if len(parts) == 1 {
		return value
	}
	if nested, ok := value.(map[string]any); ok {
		return nested[parts[1]]
	}
	if nested, ok := value.([]any); ok {
		for _, item := range nested {
			if pair, ok := item.(map[string]any); ok && textValue(pair["key"]) == parts[1] {
				return pair["value"]
			}
		}
	}
	if nested, ok := value.([]Row); ok {
		for _, pair := range nested {
			if textValue(pair["key"]) == parts[1] {
				return pair["value"]
			}
		}
	}
	return nil
}
func (r *RPC) List(name string, request Row) ([]Row, int, error) {
	m, err := r.method(name)
	if err != nil {
		return nil, 0, err
	}
	params := Row{}
	for k, v := range request {
		params[k] = v
	}
	hasPage := m.Input().Fields().ByName("page") != nil
	if hasPage {
		params["page"] = 1
		params["page_size"] = 1000
	}
	var rows []Row
	total := 0
	for page := 1; page <= 100; page++ {
		if hasPage {
			params["page"] = page
		}
		response, err := r.Call(name, params)
		if err != nil {
			return nil, 0, err
		}
		items, _ := response["data"].([]any)
		if items == nil {
			items, _ = response["items"].([]any)
		}
		for _, item := range items {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
		total = number(response["total"])
		if total == 0 {
			total = len(rows)
		}
		if !hasPage || len(items) == 0 || len(rows) >= total {
			return rows, total, nil
		}
	}
	return nil, 0, fmt.Errorf("%s exceeds bounded pagination", name)
}

// Ensure inserts only missing natural keys and verifies each returned row by reading it back.
func (run *SeedRun) Ensure(rpc *RPC, entity, listMethod, createMethod, key string, params Row, desired []Row) ([]Row, error) {
	existing, before, err := rpc.List(listMethod, params)
	if err != nil {
		return nil, err
	}
	byKey := map[string]Row{}
	for _, row := range existing {
		byKey[textValue(rowValue(row, key))] = row
	}
	added := 0
	for _, row := range desired {
		value := textValue(rowValue(row, key))
		if value == "" {
			return nil, fmt.Errorf("%s missing natural key %s", entity, key)
		}
		if _, ok := byKey[value]; ok {
			continue
		}
		if run.Apply {
			if _, err = rpc.Call(createMethod, row); err != nil {
				return nil, fmt.Errorf("%s %q: %w", entity, value, err)
			}
		}
		added++
	}
	afterRows, after, err := rpc.List(listMethod, params)
	if err != nil {
		return nil, err
	}
	byKey = map[string]Row{}
	for _, row := range afterRows {
		byKey[textValue(rowValue(row, key))] = row
	}
	selected := []Row{}
	for _, row := range desired {
		if actual, ok := byKey[textValue(rowValue(row, key))]; ok {
			selected = append(selected, actual)
		}
	}
	if run.Apply && len(selected) != len(desired) {
		return nil, fmt.Errorf("%s read-back: %d of %d", entity, len(selected), len(desired))
	}
	run.Results = append(run.Results, EntityResult{Entity: entity, Before: before, Added: added, After: after, Demo: len(selected)})
	fmt.Printf("%s: existing=%d added/planned=%d total=%d demo=%d\n", entity, before, added, after, len(selected))
	return selected, nil
}

func main() {
	apply := flag.Bool("apply", false, "create missing demo rows (default is read only)")
	count := flag.Int("count", 30, "rows per list")
	modules := flag.String("modules", "core,cmdb,io,ops,job", "comma-separated module names")
	out := flag.String("out", "logs/demo-data", "diagnostic report directory")
	inspect := flag.Bool("inspect", false, "print current real menu and operator metadata")
	flag.Parse()
	if *count < 20 || *count > 30 {
		panic("count must be between 20 and 30")
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(keys.TenantIDKey.String(), "1"))
	run := &SeedRun{Context: ctx, Apply: *apply, Count: *count}
	coreRPC, err := run.OpenRPC("127.0.0.1:9100", "core.Core")
	if err != nil {
		panic(err)
	}
	users, _, err := coreRPC.List("getUserList", Row{"username": "admin"})
	if err != nil {
		panic(err)
	}
	var admin Row
	for _, user := range users {
		if textValue(user["username"]) == "admin" {
			admin = user
		}
	}
	if admin == nil {
		panic("default tenant admin is required")
	}
	roleCodes := []string{}
	if codes, ok := admin["role_codes"].([]any); ok {
		for _, code := range codes {
			roleCodes = append(roleCodes, textValue(code))
		}
	}
	if len(roleCodes) == 0 {
		panic("operator has no role metadata")
	}
	run.Context = metadata.NewOutgoingContext(context.Background(), metadata.Pairs(keys.TenantIDKey.String(), "1", keys.UserIDKey.String(), textValue(admin["id"]), keys.UsernameKey.String(), "admin", keys.DeptIDKey.String(), textValue(admin["department_id"]), keys.RoleCodesKey.String(), strings.Join(roleCodes, ","), keys.DataScopeKey.String(), "1"))
	if err = os.MkdirAll(*out, 0700); err != nil {
		panic(err)
	}
	completed := false
	defer func() {
		body, _ := json.MarshalIndent(Row{"completed": completed, "applied": *apply, "tenant_id": 1, "count": *count, "modules": *modules, "created_at": time.Now().Format(time.RFC3339), "entities": run.Results}, "", "  ")
		if reportErr := os.WriteFile(filepath.Join(*out, "seed-report.json"), body, 0600); reportErr != nil {
			fmt.Fprintln(os.Stderr, "could not save seed report")
		}
	}()
	if *inspect {
		menus, _, err := coreRPC.List("getMenuList", Row{})
		if err != nil {
			panic(err)
		}
		clean := []Row{}
		for _, m := range menus {
			clean = append(clean, Row{"id": m["id"], "parent_id": m["parent_id"], "name": m["name"], "path": m["path"], "component": m["component"], "service_name": m["service_name"], "menu_type": m["menu_type"], "meta": m["meta"]})
		}
		body, _ := json.MarshalIndent(clean, "", "  ")
		_ = os.WriteFile(filepath.Join(*out, "menu-inventory.json"), body, 0600)
		fmt.Println(string(body))
		return
	}
	coreRPC.Close()
	for _, name := range strings.Split(*modules, ",") {
		fn, ok := seeders[name]
		if !ok {
			panic("seeder not available: " + name)
		}
		if err = fn(run); err != nil {
			panic(err)
		}
	}
	completed = true
}
