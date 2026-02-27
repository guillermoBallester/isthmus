package policy

import "github.com/guillermoBallester/isthmus/internal/core/port"

// MergeTableDetail enriches a TableDetail with business context from the policy.
// YAML descriptions are only applied when the existing Postgres comment is empty,
// so operator-set COMMENT ON values always take precedence.
func MergeTableDetail(detail *port.TableDetail, ctx ContextConfig) {
	if detail == nil {
		return
	}

	key := detail.Schema + "." + detail.Name
	tc, ok := ctx.Tables[key]
	if !ok {
		return
	}

	if detail.Comment == "" && tc.Description != "" {
		detail.Comment = tc.Description
	}

	for i, col := range detail.Columns {
		if cc, ok := tc.Columns[col.Name]; ok && col.Comment == "" && cc.Description != "" {
			detail.Columns[i].Comment = cc.Description
		}
	}
}

// MergeTableInfoList enriches a list of TableInfo with business context.
// Same precedence rule: YAML descriptions only fill empty comments.
func MergeTableInfoList(tables []port.TableInfo, ctx ContextConfig) {
	for i, t := range tables {
		key := t.Schema + "." + t.Name
		if tc, ok := ctx.Tables[key]; ok && t.Comment == "" && tc.Description != "" {
			tables[i].Comment = tc.Description
		}
	}
}

// MaskSpec extracts a column-name → mask-type map from the policy for use in query masking.
func MaskSpec(ctx ContextConfig) map[string]string {
	spec := make(map[string]string)
	for _, tc := range ctx.Tables {
		for col, cc := range tc.Columns {
			if cc.Mask != "" {
				spec[col] = cc.Mask
			}
		}
	}
	return spec
}
