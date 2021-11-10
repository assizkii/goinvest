package invest

// Validator validates types
type Validator interface {
	Validate() error
}
