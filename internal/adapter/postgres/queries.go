package postgres

// queryListSchemas has one %s placeholder for the schema filter clause.
const queryListSchemas = `
	SELECT s.schema_name
	FROM information_schema.schemata s
	WHERE %s
	ORDER BY s.schema_name`

// queryListTables has one %s placeholder for the schema filter clause.
// Returns enhanced info: total_bytes, column_count, has_indexes.
const queryListTables = `
	SELECT
		t.table_schema,
		t.table_name,
		CASE t.table_type
			WHEN 'BASE TABLE' THEN 'table'
			WHEN 'VIEW' THEN 'view'
			ELSE lower(t.table_type)
		END AS type,
		COALESCE(s.n_live_tup, 0) AS row_estimate,
		CASE WHEN t.table_type = 'BASE TABLE' THEN
			COALESCE(pg_total_relation_size(
				(quote_ident(t.table_schema) || '.' || quote_ident(t.table_name))::regclass
			), 0)
		ELSE 0
		END AS total_bytes,
		CASE WHEN t.table_type = 'BASE TABLE' THEN
			pg_size_pretty(COALESCE(pg_total_relation_size(
				(quote_ident(t.table_schema) || '.' || quote_ident(t.table_name))::regclass
			), 0))
		ELSE '0 bytes'
		END AS size_human,
		(SELECT count(*)::int FROM information_schema.columns c
		 WHERE c.table_schema = t.table_schema AND c.table_name = t.table_name
		) AS column_count,
		EXISTS(
			SELECT 1 FROM pg_indexes pgi
			WHERE pgi.schemaname = t.table_schema AND pgi.tablename = t.table_name
		) AS has_indexes,
		COALESCE(pg_catalog.obj_description(
			(quote_ident(t.table_schema) || '.' || quote_ident(t.table_name))::regclass, 'pg_class'
		), '') AS comment
	FROM information_schema.tables t
	LEFT JOIN pg_stat_user_tables s
		ON s.schemaname = t.table_schema AND s.relname = t.table_name
	WHERE %s
		AND t.table_type IN ('BASE TABLE', 'VIEW')
	ORDER BY t.table_schema, t.table_name`

// queryTableMeta has one %s placeholder for the schema filter clause.
// $1 is always table_name; schema filter params start at $2.
const queryTableMeta = `
	SELECT t.table_schema,
		   COALESCE(pg_catalog.obj_description(
			   (quote_ident(t.table_schema) || '.' || quote_ident(t.table_name))::regclass, 'pg_class'
		   ), '')
	FROM information_schema.tables t
	WHERE t.table_name = $1
		AND %s
	LIMIT 1`

// queryTableComment fetches the comment for a table with a known schema.
// $1 is schema_name, $2 is table_name.
const queryTableComment = `
	SELECT COALESCE(pg_catalog.obj_description(
		(quote_ident($1) || '.' || quote_ident($2))::regclass, 'pg_class'
	), '')`

const queryColumns = `
	SELECT
		c.column_name,
		c.data_type,
		c.is_nullable = 'YES',
		COALESCE(c.column_default, ''),
		COALESCE(pg_catalog.col_description(
			(quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass,
			c.ordinal_position
		), '')
	FROM information_schema.columns c
	WHERE c.table_schema = $1 AND c.table_name = $2
	ORDER BY c.ordinal_position`

const queryPrimaryKeys = `
	SELECT a.attname
	FROM pg_index i
	JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
	WHERE i.indrelid = (quote_ident($1) || '.' || quote_ident($2))::regclass
		AND i.indisprimary`

const queryForeignKeys = `
	SELECT
		tc.constraint_name,
		kcu.column_name,
		ccu.table_name AS referenced_table,
		ccu.column_name AS referenced_column
	FROM information_schema.table_constraints tc
	JOIN information_schema.key_column_usage kcu
		ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
	JOIN information_schema.constraint_column_usage ccu
		ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
	WHERE tc.constraint_type = 'FOREIGN KEY'
		AND tc.table_schema = $1
		AND tc.table_name = $2`

const queryIndexes = `
	SELECT
		indexname,
		indexdef,
		i.indisunique
	FROM pg_indexes pgi
	JOIN pg_class c ON c.relname = pgi.indexname
	JOIN pg_index i ON i.indexrelid = c.oid
	WHERE pgi.schemaname = $1 AND pgi.tablename = $2`

// --- Schema Profiler queries ---

// queryColumnStats fetches pg_stats data for all columns in a table.
// $1 = schema, $2 = table_name.
const queryColumnStats = `
	SELECT
		s.attname,
		s.null_frac,
		s.n_distinct,
		s.most_common_vals::text,
		s.most_common_freqs::text,
		s.histogram_bounds::text
	FROM pg_stats s
	WHERE s.schemaname = $1 AND s.tablename = $2
	ORDER BY s.attname`

// queryCheckConstraints fetches CHECK constraints for a table.
// $1 = schema, $2 = table_name.
const queryCheckConstraints = `
	SELECT
		c.conname,
		pg_get_constraintdef(c.oid)
	FROM pg_constraint c
	JOIN pg_class r ON r.oid = c.conrelid
	JOIN pg_namespace n ON n.oid = r.relnamespace
	WHERE n.nspname = $1 AND r.relname = $2 AND c.contype = 'c'
	ORDER BY c.conname`

// queryTableSize fetches row estimate, total relation size, and human-readable size.
// $1 = schema, $2 = table_name.
const queryTableSize = `
	SELECT
		COALESCE(c.reltuples::bigint, 0),
		COALESCE(pg_total_relation_size(c.oid), 0),
		pg_size_pretty(COALESCE(pg_total_relation_size(c.oid), 0))
	FROM pg_class c
	JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE n.nspname = $1 AND c.relname = $2`

// queryStatsAge fetches the timestamp of the last ANALYZE for a table.
// $1 = schema, $2 = table_name.
const queryStatsAge = `
	SELECT COALESCE(last_autoanalyze, last_analyze)
	FROM pg_stat_user_tables
	WHERE schemaname = $1 AND relname = $2`

// --- Profile table queries (deep analysis) ---

// queryTableSizeBreakdown fetches disk size breakdown: table, indexes, TOAST.
// $1 = schema, $2 = table_name.
const queryTableSizeBreakdown = `
	SELECT
		COALESCE(c.reltuples::bigint, 0) AS row_estimate,
		COALESCE(pg_total_relation_size(c.oid), 0) AS total_bytes,
		COALESCE(pg_relation_size(c.oid), 0) AS table_bytes,
		COALESCE(pg_indexes_size(c.oid), 0) AS index_bytes,
		COALESCE(pg_total_relation_size(c.reltoastrelid), 0) AS toast_bytes,
		pg_size_pretty(COALESCE(pg_total_relation_size(c.oid), 0)) AS size_human
	FROM pg_class c
	JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE n.nspname = $1 AND c.relname = $2`

// queryIndexUsage fetches usage statistics for all indexes on a table.
// $1 = schema, $2 = table_name.
const queryIndexUsage = `
	SELECT
		s.indexrelname AS index_name,
		COALESCE(s.idx_scan, 0) AS scans,
		COALESCE(pg_relation_size(s.indexrelid), 0) AS size_bytes,
		pg_size_pretty(COALESCE(pg_relation_size(s.indexrelid), 0)) AS size_human
	FROM pg_stat_user_indexes s
	WHERE s.schemaname = $1 AND s.relname = $2
	ORDER BY s.indexrelname`

// --- Profiler FK inference queries (dynamically filtered) ---

// queryResolveSchema resolves the schema for a table by name.
// $1 = table_name; schema filter placeholder at %s starts at $2.
const queryResolveSchema = `
	SELECT n.nspname
	FROM pg_class c
	JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE c.relname = $1 AND c.relkind IN ('r', 'p') AND %s
	LIMIT 1`

// queryProfilerColumns fetches column names and types for a table.
// $1 = schema, $2 = table_name.
const queryProfilerColumns = `
	SELECT a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod)
	FROM pg_attribute a
	JOIN pg_class c ON c.oid = a.attrelid
	JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
	ORDER BY a.attnum`

// queryExplicitFKColumns fetches column names that have explicit FK constraints.
// $1 = schema, $2 = table_name.
const queryExplicitFKColumns = `
	SELECT kcu.column_name
	FROM information_schema.table_constraints tc
	JOIN information_schema.key_column_usage kcu
		ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
	WHERE tc.constraint_type = 'FOREIGN KEY'
		AND tc.table_schema = $1 AND tc.table_name = $2`

// queryPKIndex fetches all primary key columns across schemas for FK inference.
// Schema filter placeholder at %s starts at $1.
const queryPKIndex = `
	SELECT c.relname, a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod)
	FROM pg_index i
	JOIN pg_class c ON c.oid = i.indrelid
	JOIN pg_namespace n ON n.oid = c.relnamespace
	JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
	WHERE i.indisprimary AND %s`
