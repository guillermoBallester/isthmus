package domain

// CardinalityClass describes the distribution shape of a column's values.
type CardinalityClass string

const (
	CardinalityUnique          CardinalityClass = "unique"
	CardinalityNearUnique      CardinalityClass = "near_unique"
	CardinalityHighCardinality CardinalityClass = "high_cardinality"
	CardinalityLowCardinality  CardinalityClass = "low_cardinality"
	CardinalityEnumLike        CardinalityClass = "enum_like"
)

// ClassifyByDistinctCount determines the cardinality class from absolute
// distinct and total row counts. This is database-agnostic — adapters must
// convert DB-specific statistics (e.g. pg_stats n_distinct) to absolute
// counts before calling this function.
func ClassifyByDistinctCount(distinctCount int64, totalRows int64) CardinalityClass {
	if totalRows > 0 && distinctCount == totalRows {
		return CardinalityUnique
	}

	if totalRows > 0 {
		ratio := float64(distinctCount) / float64(totalRows)
		if ratio >= 0.9 {
			return CardinalityNearUnique
		}
	}

	if distinctCount <= 20 {
		return CardinalityEnumLike
	}
	if distinctCount <= 200 {
		return CardinalityLowCardinality
	}
	return CardinalityHighCardinality
}
