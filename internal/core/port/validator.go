package port

// QueryValidator validates SQL statements before execution.
type QueryValidator interface {
	Validate(sql string) error
}
