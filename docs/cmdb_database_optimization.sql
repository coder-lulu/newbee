-- NewBee CMDB 数据库索引优化脚本
-- 基于CLAUDE.md编码准则和多专家分析结果
-- 执行前请先备份数据库！

-- ============================================
-- 1. CI主表索引优化
-- ============================================

-- CI主表核心索引（基于tenant_id的多租户查询优化）
ALTER TABLE cmdb_cis 
ADD INDEX IF NOT EXISTS idx_tenant_type_status (tenant_id, type_id, status);

ALTER TABLE cmdb_cis 
ADD INDEX IF NOT EXISTS idx_tenant_dept_created (tenant_id, department_id, created_at);

ALTER TABLE cmdb_cis 
ADD INDEX IF NOT EXISTS idx_tenant_created_desc (tenant_id, created_at DESC);

ALTER TABLE cmdb_cis 
ADD INDEX IF NOT EXISTS idx_type_status_available (type_id, status, available);

-- 心跳监控专用索引
ALTER TABLE cmdb_cis
ADD INDEX IF NOT EXISTS idx_tenant_heartbeat (tenant_id, heartbeat DESC);

-- 创建者查询索引
ALTER TABLE cmdb_cis
ADD INDEX IF NOT EXISTS idx_tenant_created_by (tenant_id, created_by);

-- ============================================
-- 2. Value表索引优化（EAV模式核心优化）
-- ============================================

-- value_texts表索引
ALTER TABLE cmdb_value_texts
ADD INDEX IF NOT EXISTS idx_tenant_ci_attr (tenant_id, ci_id, attr_id);

ALTER TABLE cmdb_value_texts
ADD INDEX IF NOT EXISTS idx_tenant_attr_value (tenant_id, attr_id, value(100));

-- CI属性唯一性约束（防止重复属性值）
ALTER TABLE cmdb_value_texts
ADD UNIQUE INDEX IF NOT EXISTS uk_ci_attr (ci_id, attr_id);

-- 属性值模糊搜索索引
ALTER TABLE cmdb_value_texts
ADD INDEX IF NOT EXISTS idx_tenant_value_search (tenant_id, value(50));

-- value_integers表索引
ALTER TABLE cmdb_value_integers
ADD INDEX IF NOT EXISTS idx_tenant_ci_attr (tenant_id, ci_id, attr_id);

ALTER TABLE cmdb_value_integers
ADD INDEX IF NOT EXISTS idx_tenant_attr_value (tenant_id, attr_id, value);

ALTER TABLE cmdb_value_integers
ADD UNIQUE INDEX IF NOT EXISTS uk_ci_attr (ci_id, attr_id);

-- 数值范围查询索引
ALTER TABLE cmdb_value_integers
ADD INDEX IF NOT EXISTS idx_value_range (tenant_id, attr_id, value);

-- value_floats表索引
ALTER TABLE cmdb_value_floats
ADD INDEX IF NOT EXISTS idx_tenant_ci_attr (tenant_id, ci_id, attr_id);

ALTER TABLE cmdb_value_floats
ADD INDEX IF NOT EXISTS idx_tenant_attr_value (tenant_id, attr_id, value);

ALTER TABLE cmdb_value_floats
ADD UNIQUE INDEX IF NOT EXISTS uk_ci_attr (ci_id, attr_id);

-- value_datetimes表索引
ALTER TABLE cmdb_value_datetimes
ADD INDEX IF NOT EXISTS idx_tenant_ci_attr (tenant_id, ci_id, attr_id);

ALTER TABLE cmdb_value_datetimes
ADD INDEX IF NOT EXISTS idx_tenant_attr_datetime (tenant_id, attr_id, value);

ALTER TABLE cmdb_value_datetimes
ADD UNIQUE INDEX IF NOT EXISTS uk_ci_attr (ci_id, attr_id);

-- 时间范围查询索引
ALTER TABLE cmdb_value_datetimes
ADD INDEX IF NOT EXISTS idx_datetime_range (tenant_id, attr_id, value);

-- value_jsons表索引
ALTER TABLE cmdb_value_jsons
ADD INDEX IF NOT EXISTS idx_tenant_ci_attr (tenant_id, ci_id, attr_id);

ALTER TABLE cmdb_value_jsons
ADD UNIQUE INDEX IF NOT EXISTS uk_ci_attr (ci_id, attr_id);

-- JSON字段的虚拟列索引（需要MySQL 8.0+，如果是MySQL 5.7请注释掉）
-- ALTER TABLE cmdb_value_jsons
-- ADD COLUMN hostname_virtual VARCHAR(255) AS (JSON_UNQUOTE(JSON_EXTRACT(value, '$.hostname'))),
-- ADD INDEX idx_hostname_virtual (tenant_id, hostname_virtual);

-- ============================================
-- 3. 关系表索引优化
-- ============================================

-- CI关系表核心索引
ALTER TABLE cmdb_ci_relations
ADD INDEX IF NOT EXISTS idx_tenant_first_second (tenant_id, first_ci_id, second_ci_id);

ALTER TABLE cmdb_ci_relations
ADD INDEX IF NOT EXISTS idx_tenant_second_first (tenant_id, second_ci_id, first_ci_id);

ALTER TABLE cmdb_ci_relations
ADD INDEX IF NOT EXISTS idx_tenant_relation_type (tenant_id, relation_type_id);

-- 关系类型查询索引
ALTER TABLE cmdb_ci_relations
ADD INDEX IF NOT EXISTS idx_first_relation_type (first_ci_id, relation_type_id);

-- 祖先路径索引（支持关系网络遍历）
ALTER TABLE cmdb_ci_relations
ADD INDEX IF NOT EXISTS idx_ancestor_ids (ancestor_ids);

-- ============================================
-- 4. 属性定义表索引优化
-- ============================================

-- 属性表租户查询索引
ALTER TABLE cmdb_attributes
ADD INDEX IF NOT EXISTS idx_tenant_name (tenant_id, name);

ALTER TABLE cmdb_attributes
ADD INDEX IF NOT EXISTS idx_tenant_type (tenant_id, value_type);

-- 属性引用类型索引
ALTER TABLE cmdb_attributes
ADD INDEX IF NOT EXISTS idx_tenant_ref_type (tenant_id, reference_type_id);

-- ============================================
-- 5. CI类型相关表索引优化
-- ============================================

-- CI类型表索引
ALTER TABLE cmdb_ci_types
ADD INDEX IF NOT EXISTS idx_tenant_name (tenant_id, name);

ALTER TABLE cmdb_ci_types
ADD INDEX IF NOT EXISTS idx_tenant_category (tenant_id, category);

ALTER TABLE cmdb_ci_types
ADD INDEX IF NOT EXISTS idx_tenant_status (tenant_id, status);

-- CI类型属性关联表索引
ALTER TABLE cmdb_ci_type_attributes
ADD INDEX IF NOT EXISTS idx_tenant_type_attr (tenant_id, type_id, attribute_id);

ALTER TABLE cmdb_ci_type_attributes
ADD INDEX IF NOT EXISTS idx_tenant_attr_type (tenant_id, attribute_id, type_id);

-- 属性排序索引
ALTER TABLE cmdb_ci_type_attributes
ADD INDEX IF NOT EXISTS idx_tenant_type_sort (tenant_id, type_id, sort);

-- ============================================
-- 6. 权限相关表索引优化
-- ============================================

-- CI权限表索引
ALTER TABLE cmdb_ci_permissions
ADD INDEX IF NOT EXISTS idx_tenant_type_level (tenant_id, ci_type_id, permission_level);

ALTER TABLE cmdb_ci_permissions
ADD INDEX IF NOT EXISTS idx_tenant_user_type (tenant_id, user_id, ci_type_id);

ALTER TABLE cmdb_ci_permissions
ADD INDEX IF NOT EXISTS idx_tenant_role_type (tenant_id, role_id, ci_type_id);

-- ============================================
-- 7. 选项值表索引优化
-- ============================================

-- 文本选项表索引
ALTER TABLE cmdb_choice_texts
ADD INDEX IF NOT EXISTS idx_tenant_attr_value (tenant_id, attribute_id, value);

-- 整数选项表索引
ALTER TABLE cmdb_choice_integers
ADD INDEX IF NOT EXISTS idx_tenant_attr_value (tenant_id, attribute_id, value);

-- 浮点选项表索引
ALTER TABLE cmdb_choice_floats
ADD INDEX IF NOT EXISTS idx_tenant_attr_value (tenant_id, attribute_id, value);

-- ============================================
-- 8. 性能监控查询
-- ============================================

-- 创建视图用于监控分片数据分布
CREATE OR REPLACE VIEW v_tenant_data_distribution AS
SELECT 
    tenant_id,
    COUNT(*) as ci_count,
    (SELECT COUNT(*) FROM cmdb_value_texts vt WHERE vt.tenant_id = c.tenant_id) as text_values_count,
    (SELECT COUNT(*) FROM cmdb_value_integers vi WHERE vi.tenant_id = c.tenant_id) as int_values_count,
    (SELECT COUNT(*) FROM cmdb_value_floats vf WHERE vf.tenant_id = c.tenant_id) as float_values_count,
    (SELECT COUNT(*) FROM cmdb_ci_relations cr WHERE cr.tenant_id = c.tenant_id) as relations_count,
    ROUND(COUNT(*) * 100.0 / (SELECT COUNT(*) FROM cmdb_cis), 2) as percentage
FROM cmdb_cis c 
GROUP BY tenant_id
ORDER BY ci_count DESC;

-- 创建视图用于监控索引使用情况
CREATE OR REPLACE VIEW v_index_usage_stats AS
SELECT 
    table_schema,
    table_name,
    index_name,
    cardinality,
    non_unique,
    ROUND((cardinality / (SELECT COUNT(*) FROM information_schema.tables t2 WHERE t2.table_schema = s.table_schema AND t2.table_name = s.table_name)) * 100, 2) as selectivity_percent
FROM information_schema.statistics s
WHERE table_schema = DATABASE()
    AND table_name LIKE 'cmdb_%'
    AND index_name != 'PRIMARY'
ORDER BY table_name, cardinality DESC;

-- ============================================
-- 9. 查询性能测试SQL
-- ============================================

-- 测试1：CI列表查询性能测试
-- EXPLAIN ANALYZE 
-- SELECT * FROM cmdb_cis 
-- WHERE tenant_id = 1 AND type_id = 100 
-- ORDER BY created_at DESC LIMIT 50;

-- 测试2：CI详情查询性能测试
-- EXPLAIN ANALYZE
-- SELECT c.*, vt.value as hostname, vi.value as cpu_count
-- FROM cmdb_cis c
-- LEFT JOIN cmdb_value_texts vt ON c.id = vt.ci_id AND vt.attr_id = 1
-- LEFT JOIN cmdb_value_integers vi ON c.id = vi.ci_id AND vi.attr_id = 2
-- WHERE c.id = 12345 AND c.tenant_id = 1;

-- 测试3：属性值搜索性能测试
-- EXPLAIN ANALYZE
-- SELECT c.* FROM cmdb_cis c
-- JOIN cmdb_value_texts vt ON c.id = vt.ci_id
-- WHERE c.tenant_id = 1 AND vt.attr_id = 1 AND vt.value LIKE '%web%';

-- ============================================
-- 10. 索引维护脚本
-- ============================================

-- 分析表统计信息（建议定期执行）
-- ANALYZE TABLE cmdb_cis, cmdb_value_texts, cmdb_value_integers, cmdb_value_floats, 
--                cmdb_value_datetimes, cmdb_value_jsons, cmdb_ci_relations, cmdb_attributes;

-- 检查表碎片化情况
-- SELECT 
--     table_name,
--     ROUND(((data_length + index_length) / 1024 / 1024), 2) as size_mb,
--     ROUND((data_free / 1024 / 1024), 2) as free_mb,
--     ROUND((data_free / (data_length + index_length)) * 100, 2) as fragmentation_percent
-- FROM information_schema.tables
-- WHERE table_schema = DATABASE()
--     AND table_name LIKE 'cmdb_%'
--     AND data_free > 0
-- ORDER BY fragmentation_percent DESC;

-- ============================================
-- 执行完成提示
-- ============================================
SELECT '数据库索引优化完成！建议执行ANALYZE TABLE来更新统计信息。' as message;