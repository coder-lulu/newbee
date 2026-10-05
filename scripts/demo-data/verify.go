package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coder-lulu/newbee-common/v2/middleware/keys"
	"github.com/coder-lulu/newbee-common/v2/utils/jwt"
	"google.golang.org/grpc/metadata"
)

func init() { seeders["verify"] = verifyDemoAPIs }

type apiCheck struct {
	entity, method, path string
	params               Row
	minimum              int
	tree, stats          bool
	note                 string
}

type apiCheckResult struct {
	Entity  string `json:"entity"`
	Path    string `json:"path"`
	Method  string `json:"method"`
	HTTP    int    `json:"http"`
	Code    string `json:"code"`
	Total   int    `json:"total"`
	Rows    int    `json:"rows"`
	Minimum int    `json:"minimum_total"`
	Passed  bool   `json:"passed"`
	Note    string `json:"note,omitempty"`
	Failure string `json:"failure,omitempty"`
}

func verifyDemoAPIs(run *SeedRun) error {
	data, err := os.ReadFile("logs/local-run/private-runtime.json")
	if err != nil {
		return fmt.Errorf("local JWT runtime unavailable")
	}
	var config struct {
		Database string `json:"database"`
		Secret   string `json:"jwt_secret"`
	}
	if json.Unmarshal(data, &config) != nil || config.Database != "newbee" || config.Secret == "" {
		return fmt.Errorf("local newbee JWT configuration required")
	}
	md, ok := metadata.FromOutgoingContext(run.Context)
	if !ok {
		return fmt.Errorf("authenticated operator metadata missing")
	}
	value := func(key string) string {
		v := md.Get(key)
		if len(v) != 1 {
			return ""
		}
		return v[0]
	}
	uid, tid, username := value(keys.UserIDKey.String()), value(keys.TenantIDKey.String()), value(keys.UsernameKey.String())
	roles := value(keys.RoleCodesKey.String())
	if uid == "" || tid != "1" || username != "admin" || roles == "" {
		return fmt.Errorf("verification requires real local tenant 1 admin metadata")
	}
	dept, err := strconv.ParseUint(value(keys.DeptIDKey.String()), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid operator department")
	}
	token, err := jwt.NewJwtToken(config.Secret, time.Now().Unix(), 900, jwt.WithOption(keys.JWTUserID, uid), jwt.WithOption(keys.JWTTenantID, uint64(1)), jwt.WithOption(keys.JWTUsername, username), jwt.WithOption(keys.JWTDeptID, dept), jwt.WithOption(keys.JWTRoleCodes, roles))
	if err != nil {
		return fmt.Errorf("local verification JWT creation failed")
	}
	checks := []apiCheck{}
	add := func(entity, method, path string, minimum int) {
		checks = append(checks, apiCheck{entity: entity, method: method, path: path, minimum: minimum})
	}
	for _, name := range []string{"user", "role", "department", "position", "dictionary", "configuration", "oauth_provider", "tenant", "task", "audit-log"} {
		add("core."+name, "POST", "/sys-api/"+name+"/list", run.Count)
	}
	for i := range checks {
		if checks[i].entity == "core.department" {
			checks[i].tree = true
		}
	}
	for _, name := range []string{"api", "menu", "token"} {
		method := "POST"
		if name == "menu" {
			method = "GET"
		}
		checks = append(checks, apiCheck{entity: "core." + name, method: method, path: "/sys-api/" + name + "/list", minimum: 1, tree: name == "menu", note: "系统目录/真实会话观察项，不生成虚假令牌或菜单"})
	}
	checks = append(checks, apiCheck{entity: "core.oauth_statistics", method: "POST", path: "/sys-api/oauth/statistics", stats: true, note: "真实API为POST；现有UI封装GET不匹配需另修。统计不要求30行"}, apiCheck{entity: "core.audit_statistics", method: "POST", path: "/sys-api/audit-log/stats", stats: true, note: "统计数组，不要求30行"})
	for _, name := range []string{"attribute", "ci_type", "relation_type", "ci_type_relation", "ci_permission"} {
		add("cmdb."+name, "POST", "/cmdb-api/"+name+"/list", run.Count)
	}
	checks = append(checks, apiCheck{entity: "cmdb.type_groups", method: "POST", path: "/cmdb-api/ci_type_group/list", minimum: 6, note: "分组→模型目录，设计为6个业务分组"})
	for _, name := range []string{"input_task", "output_task", "discovery_pool", "discovery_template", "data_target", "field_mapping", "config", "worker_metrics"} {
		method := "POST"
		if name == "output_task" || name == "field_mapping" {
			method = "GET"
		}
		add("io."+name, method, "/io-api/"+name+"/list", run.Count)
	}
	add("io.config_audit", "POST", "/io-api/config/audit_log/list", run.Count)
	add("io.change_history", "POST", "/io-api/change-history/list", run.Count)
	add("io.lifecycle_state", "POST", "/io-api/lifecycle-state/list", run.Count)
	add("io.task_logs", "GET", "/io-api/task_log/list", run.Count)
	add("io.mapping_logs", "GET", "/io-api/mapping_log/list", run.Count)
	checks = append(checks, apiCheck{entity: "io.provider_catalog", method: "POST", path: "/io-api/discoveryprovider/list_discovery_providers", minimum: run.Count, note: "内置与数据库Provider合并目录；数组响应按实际条目计数，前端再分页"})
	for _, name := range []string{"proxy", "proxygroup", "agent", "agentgroup", "script", "script/category"} {
		add("ops."+strings.ReplaceAll(name, "/", "_"), "GET", "/ops-api/"+name+"/list", run.Count)
	}
	add("ops.sessions", "GET", "/ops-center-api/ops/session/list", run.Count)
	add("ops.profiles", "GET", "/ops-center-api/ops/profile/list", run.Count)
	checks = append(checks, apiCheck{entity: "ops.worker_metrics", method: "GET", path: "/ops-api/proxy/metrics", minimum: run.Count, params: Row{"proxy_id": "demo-proxy-01", "limit": 1000}, note: "真实Worker历史指标，数组由前端分页"}, apiCheck{entity: "ops.worker_statistics", method: "GET", path: "/ops-api/proxy/metrics/stats", stats: true, params: Row{"proxy_id": "demo-proxy-01"}, note: "统计对象，不要求30行"})
	// CI pages are scoped to a model in the UI. Check every model separately.
	cmdb, e := run.OpenRPC("127.0.0.1:9200", "cmdb.Cmdb")
	if e == nil {
		models, _, listErr := cmdb.List("getCiTypeList", Row{})
		cmdb.Close()
		if listErr == nil {
			for _, model := range models {
				checks = append(checks, apiCheck{entity: fmt.Sprintf("cmdb.cis.type_%v", model["id"]), method: "POST", path: "/cmdb-api/cis/list", minimum: run.Count, params: Row{"typeId": number(model["id"]), "withAttributes": true}})
			}
		} else {
			e = listErr
		}
	}
	results := []apiCheckResult{}
	if e != nil {
		results = append(results, apiCheckResult{Entity: "cmdb.model_inventory", Failure: "model inventory RPC failed"})
	}
	job, jobErr := run.OpenRPC("127.0.0.1:9105", "job.Job")
	if jobErr == nil {
		tasks, _, listErr := job.List("getTaskList", Row{})
		job.Close()
		if listErr != nil {
			jobErr = listErr
		} else {
			for _, task := range tasks {
				if textValue(task["task_group"]) == "newbee-demo-disabled" {
					checks = append(checks, apiCheck{entity: fmt.Sprintf("core.task_log.task_%v", task["id"]), method: "POST", path: "/sys-api/task_log/list", minimum: run.Count, params: Row{"taskId": number(task["id"])}})
				}
			}
		}
	}
	if jobErr != nil {
		results = append(results, apiCheckResult{Entity: "job.task_inventory", Failure: "task inventory RPC failed"})
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	for _, check := range checks {
		result := runAPICheck(client, token, check)
		results = append(results, result)
		fmt.Printf("verify %s: http=%d code=%s rows=%d total=%d passed=%t\n", result.Entity, result.HTTP, result.Code, result.Rows, result.Total, result.Passed)
	}
	failed := []string{}
	for _, result := range results {
		if !result.Passed {
			failed = append(failed, result.Entity)
		}
	}
	report, err := json.MarshalIndent(Row{"created_at": time.Now().Format(time.RFC3339), "tenant_id": 1, "page_size": 20, "required_demo_total": run.Count, "checks": results, "failed": failed, "passed": len(failed) == 0}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll("logs/demo-data", 0700); err != nil {
		return err
	}
	if err = os.WriteFile("logs/demo-data/api-verification.json", report, 0600); err != nil {
		return err
	}
	if len(failed) > 0 {
		return fmt.Errorf("API verification failed for %d checks: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

func runAPICheck(client *http.Client, token string, check apiCheck) apiCheckResult {
	result := apiCheckResult{Entity: check.entity, Path: check.path, Method: check.method, Minimum: check.minimum, Note: check.note}
	params := Row{"page": 1, "pageSize": 20}
	for k, v := range check.params {
		params[k] = v
	}
	if check.entity == "ops.sessions" {
		delete(params, "pageSize")
		params["size"] = 20
	}
	target := "http://127.0.0.1:5666" + check.path
	var body io.Reader
	if check.method == "GET" {
		query := url.Values{}
		for k, v := range params {
			query.Set(k, fmt.Sprint(v))
		}
		target += "?" + query.Encode()
	} else {
		encoded, e := json.Marshal(params)
		if e != nil {
			result.Failure = "request encoding failed"
			return result
		}
		body = bytes.NewReader(encoded)
		// IO's PageInfo has form defaults as well as JSON fields. Supplying
		// the declared form pagination prevents defaults overriding JSON.
		if strings.HasPrefix(check.path, "/io-api/") {
			target += "?page=1&pageSize=20"
		}
	}
	req, err := http.NewRequest(check.method, target, body)
	if err != nil {
		result.Failure = "request construction failed"
		return result
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "zh_CN")
	response, err := client.Do(req)
	if err != nil {
		result.Failure = "local HTTP request failed"
		return result
	}
	defer response.Body.Close()
	result.HTTP = response.StatusCode
	var payload any
	decoder := json.NewDecoder(io.LimitReader(response.Body, 20<<20))
	decoder.UseNumber()
	if err = decoder.Decode(&payload); err != nil {
		result.Failure = "response is not valid JSON"
		return result
	}
	businessOK := false
	if envelope, ok := payload.(map[string]any); ok {
		if code, exists := envelope["code"]; exists {
			result.Code = fmt.Sprint(code)
			businessOK = result.Code == "0"
			payload = envelope["data"]
		}
	}
	if response.StatusCode != http.StatusOK || !businessOK {
		result.Failure = "UI transport requires HTTP 200 and code 0"
		return result
	}
	if check.stats {
		switch value := payload.(type) {
		case map[string]any:
			result.Passed = len(value) > 0
		case []any:
			result.Passed = len(value) > 0
			result.Rows = len(value)
		}
		if !result.Passed {
			result.Failure = "statistics payload is empty"
		}
		return result
	}
	rows, total, found := apiListCounts(payload, check.tree)
	result.Rows = rows
	result.Total = total
	minimumRows := check.minimum
	if minimumRows > 20 {
		minimumRows = 20
	}
	result.Passed = found && total >= check.minimum && rows >= minimumRows
	if !result.Passed {
		result.Failure = "list shape or minimum row count failed"
	}
	return result
}

func apiListCounts(payload any, tree bool) (int, int, bool) {
	if items, ok := payload.([]any); ok {
		count := len(items)
		if tree {
			for _, item := range items {
				if row, ok := item.(map[string]any); ok {
					nested, _, _ := apiListCounts(row["children"], true)
					count += nested
				}
			}
		}
		return count, count, true
	}
	if object, ok := payload.(map[string]any); ok {
		for _, key := range []string{"data", "rows", "items", "Items", "list"} {
			if nested, exists := object[key]; exists {
				rows, total, found := apiListCounts(nested, tree)
				if raw, exists := object["total"]; exists {
					total = number(raw)
				}
				if raw, exists := object["Total"]; exists {
					total = number(raw)
				}
				if tree && total < rows {
					total = rows
				}
				if found {
					return rows, total, true
				}
			}
		}
	}
	return 0, 0, false
}
