package store

type Store interface {
	SaveSpec(spec *SpecRecord) error
	GetSpec(id string) (*SpecRecord, error)
	ListSpecs() ([]*SpecRecord, error)
	DeleteSpec(id string) error

	SaveOperations(specID string, ops []*OperationRecord) error
	GetOperations(specID string) ([]*OperationRecord, error)
	UpdateOperation(specID string, op *OperationRecord) error
	DeleteOperations(specID string) error

	SaveAuth(specID string, cfg *AuthConfig) error
	GetAuth(specID string) (*AuthConfig, error)
	DeleteAuth(specID string) error

	IncrementStats(operationID string, latencyMs int64, isError bool) error
	GetAllStats() (map[string]*ToolStats, error)
	GetStats(operationID string) (*ToolStats, error)
}
