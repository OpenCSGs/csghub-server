package rebac

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Authorizer is the stable application-facing ReBAC facade.
type Authorizer interface {
	// Write validates and persists direct relationship tuples.
	// For example, the following relationship grants user-1 a direct reader
	// relationship on repository 42:
	//
	//	err := authorizer.Write(ctx, []Relationship{{
	//		Subject:  UserSubject("user-1"),
	//		Relation: RelationReader,
	//		Object:   RepositoryObject(42),
	//	}})
	Write(ctx context.Context, relationships []Relationship) error
	// Delete validates and removes direct relationship tuples.
	// For example, deleting the same relationship revokes the direct reader
	// grant from user-1 on repository 42:
	//
	//	err := authorizer.Delete(ctx, []Relationship{{
	//		Subject:  UserSubject("user-1"),
	//		Relation: RelationReader,
	//		Object:   RepositoryObject(42),
	//	}})
	Delete(ctx context.Context, relationships []Relationship) error
	// Check determines whether a subject has a relation or computed permission on an object.
	// A denial returns Allowed=false with a nil error; invalid requests and Provider failures return errors.
	// For example, the following request checks whether user-1 can read
	// repository 42 through a direct grant or inherited organization permission:
	//
	//	decision, err := authorizer.Check(ctx, CheckRequest{
	//		Subject:  UserSubject("user-1"),
	//		Relation: RepositoryCanRead,
	//		Object:   RepositoryObject(42),
	//	})
	Check(ctx context.Context, request CheckRequest) (Decision, error)
	// Authorize enforces one authorization check.
	// It returns nil when allowed, ErrDenied when denied, and the corresponding error for other failures.
	Authorize(ctx context.Context, request CheckRequest) error
	// BatchCheck evaluates independent authorization checks and returns outcomes by correlation ID.
	// Per-check errors are recorded in their outcomes; an unexecutable request returns a top-level error.
	BatchCheck(ctx context.Context, request BatchCheckRequest) (BatchCheckResult, error)
	// ListObjects lists object references a subject can access through a relation or computed permission.
	// It returns authorization objects only and does not query, sort, or paginate business data.
	ListObjects(ctx context.Context, request ListObjectsRequest) (ListObjectsResult, error)
	// ListSubjects lists subjects that have a relation or computed permission on an object.
	// Results may contain concrete subjects or object#relation usersets.
	ListSubjects(ctx context.Context, request ListSubjectsRequest) (ListSubjectsResult, error)
}

// Option configures an Authorizer without exposing the concrete Provider implementation.
type Option func(*authorizerConfig) error

type authorizerConfig struct {
	schema       *Schema
	observer     Observer
	timeout      time.Duration
	maxBatchSize int
}

type authorizer struct {
	provider     Provider
	providerName string
	validator    *Validator
	observer     Observer
	timeout      time.Duration
	maxBatchSize int
}

// NewAuthorizer creates an authorization facade independent of a concrete Provider.
func NewAuthorizer(provider Provider, options ...Option) (Authorizer, error) {
	if provider == nil {
		return nil, fmt.Errorf("%w: provider is nil", ErrInvalidRequest)
	}

	schema, err := DefaultSchema()
	if err != nil {
		return nil, err
	}
	config := authorizerConfig{
		schema:       schema,
		observer:     NoopObserver(),
		maxBatchSize: DefaultMaxBatchSize,
	}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("%w: authorizer option is nil", ErrInvalidRequest)
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}

	validator, err := NewValidator(config.schema)
	if err != nil {
		return nil, err
	}
	providerName := provider.Name()
	if providerName == "" {
		providerName = "unnamed"
	}

	return &authorizer{
		provider:     provider,
		providerName: providerName,
		validator:    validator,
		observer:     config.observer,
		timeout:      config.timeout,
		maxBatchSize: config.maxBatchSize,
	}, nil
}

// WithSchema replaces the default schema with an immutable custom schema.
func WithSchema(schema *Schema) Option {
	return func(config *authorizerConfig) error {
		if schema == nil {
			return fmt.Errorf("%w: schema is nil", ErrInvalidSchema)
		}
		config.schema = schema
		return nil
	}
}

// WithObserver configures an authorization telemetry hook.
func WithObserver(observer Observer) Option {
	return func(config *authorizerConfig) error {
		if observer == nil {
			return fmt.Errorf("%w: observer is nil", ErrInvalidRequest)
		}
		config.observer = observer
		return nil
	}
}

// WithTimeout adds an Authorizer-managed timeout to Provider calls.
// A zero timeout reuses the caller's context without adding a deadline.
func WithTimeout(timeout time.Duration) Option {
	return func(config *authorizerConfig) error {
		if timeout < 0 {
			return fmt.Errorf("%w: timeout must not be negative", ErrInvalidRequest)
		}
		config.timeout = timeout
		return nil
	}
}

// WithMaxBatchSize configures the maximum number of checks in one batch.
func WithMaxBatchSize(maxBatchSize int) Option {
	return func(config *authorizerConfig) error {
		if maxBatchSize <= 0 {
			return fmt.Errorf("%w: maximum batch size must be positive", ErrInvalidRequest)
		}
		config.maxBatchSize = maxBatchSize
		return nil
	}
}

// Write validates and persists direct relationship tuples.
func (a *authorizer) Write(ctx context.Context, relationships []Relationship) error {
	return a.mutateRelationships(ctx, OperationWrite, relationships, a.provider.Write)
}

// Delete validates and removes direct relationship tuples.
func (a *authorizer) Delete(ctx context.Context, relationships []Relationship) error {
	return a.mutateRelationships(ctx, OperationDelete, relationships, a.provider.Delete)
}

// Check validates and executes one authorization request.
func (a *authorizer) Check(ctx context.Context, request CheckRequest) (Decision, error) {
	return a.check(ctx, request, OperationCheck, false)
}

// Authorize returns ErrDenied when a valid request is not allowed.
func (a *authorizer) Authorize(ctx context.Context, request CheckRequest) error {
	_, err := a.check(ctx, request, OperationAuthorize, true)
	return err
}

// BatchCheck validates and executes a bounded set of independent authorization checks.
func (a *authorizer) BatchCheck(ctx context.Context, request BatchCheckRequest) (result BatchCheckResult, err error) {
	started := time.Now()
	observation := Observation{
		Operation:  OperationBatchCheck,
		Provider:   a.providerName,
		BatchSize:  len(request.Checks),
		ErrorClass: ErrorClassNone,
	}
	defer func() {
		observation.Duration = time.Since(started)
		observation.ErrorClass = ClassifyError(err)
		a.observer.Observe(observerContext(ctx), observation)
	}()

	if ctx == nil {
		return BatchCheckResult{}, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := a.validator.ValidateBatchCheckRequest(request, a.maxBatchSize); err != nil {
		return BatchCheckResult{}, err
	}

	providerCtx, cancel := a.providerContext(ctx)
	defer cancel()
	result, err = a.provider.BatchCheck(providerCtx, request)
	if err = normalizeProviderError(providerCtx, err); err != nil {
		return BatchCheckResult{}, err
	}
	if err := validateBatchProviderResult(request, result, &observation); err != nil {
		return BatchCheckResult{}, err
	}
	return result, nil
}

// ListObjects validates and executes an object listing query with authorization constraints.
func (a *authorizer) ListObjects(ctx context.Context, request ListObjectsRequest) (result ListObjectsResult, err error) {
	started := time.Now()
	defer func() {
		a.observe(ctx, Observation{
			Operation:  OperationListObjects,
			Provider:   a.providerName,
			ObjectType: request.ObjectType,
			Relation:   request.Relation,
			Duration:   time.Since(started),
			ErrorClass: ClassifyError(err),
		})
	}()

	if ctx == nil {
		return ListObjectsResult{}, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := a.validator.ValidateListObjectsRequest(request); err != nil {
		return ListObjectsResult{}, err
	}
	providerCtx, cancel := a.providerContext(ctx)
	defer cancel()
	result, err = a.provider.ListObjects(providerCtx, request)
	if err = normalizeProviderError(providerCtx, err); err != nil {
		return ListObjectsResult{}, err
	}
	if err := a.validateObjectResult(request.ObjectType, result.Objects); err != nil {
		return ListObjectsResult{}, err
	}
	return result, nil
}

// ListSubjects validates and executes a subject listing query with authorization constraints.
func (a *authorizer) ListSubjects(ctx context.Context, request ListSubjectsRequest) (result ListSubjectsResult, err error) {
	started := time.Now()
	defer func() {
		a.observe(ctx, Observation{
			Operation:  OperationListSubjects,
			Provider:   a.providerName,
			ObjectType: request.Object.Type,
			Relation:   request.Relation,
			Duration:   time.Since(started),
			ErrorClass: ClassifyError(err),
		})
	}()

	if ctx == nil {
		return ListSubjectsResult{}, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := a.validator.ValidateListSubjectsRequest(request); err != nil {
		return ListSubjectsResult{}, err
	}
	providerCtx, cancel := a.providerContext(ctx)
	defer cancel()
	result, err = a.provider.ListSubjects(providerCtx, request)
	if err = normalizeProviderError(providerCtx, err); err != nil {
		return ListSubjectsResult{}, err
	}
	if err := a.validateSubjectResult(request.SubjectType, result.Subjects); err != nil {
		return ListSubjectsResult{}, err
	}
	return result, nil
}

func (a *authorizer) check(
	ctx context.Context,
	request CheckRequest,
	operation Operation,
	enforce bool,
) (decision Decision, err error) {
	started := time.Now()
	decisionMade := false
	defer func() {
		a.observe(ctx, Observation{
			Operation:    operation,
			Provider:     a.providerName,
			ObjectType:   request.Object.Type,
			Relation:     request.Relation,
			Duration:     time.Since(started),
			DecisionMade: decisionMade,
			Allowed:      decisionMade && decision.Allowed,
			ErrorClass:   ClassifyError(err),
		})
	}()

	if ctx == nil {
		return Decision{}, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := a.validator.ValidateCheckRequest(request); err != nil {
		return Decision{}, err
	}
	providerCtx, cancel := a.providerContext(ctx)
	defer cancel()
	decision, err = a.provider.Check(providerCtx, request)
	if err = normalizeProviderError(providerCtx, err); err != nil {
		if errors.Is(err, ErrDenied) {
			return Decision{}, fmt.Errorf("%w: provider returned denial as an error", ErrInvalidProviderResponse)
		}
		return Decision{}, err
	}
	decisionMade = true
	if enforce && !decision.Allowed {
		return decision, fmt.Errorf("%w: %s on %s", ErrDenied, request.Relation, request.Object.Type)
	}
	return decision, nil
}

func (a *authorizer) mutateRelationships(ctx context.Context, operation Operation, relationships []Relationship, mutate func(context.Context, []Relationship) error) (err error) {
	started := time.Now()
	defer func() {
		a.observe(ctx, Observation{
			Operation:  operation,
			Provider:   a.providerName,
			Duration:   time.Since(started),
			BatchSize:  len(relationships),
			ErrorClass: ClassifyError(err),
		})
	}()

	if ctx == nil {
		return fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := a.validator.ValidateRelationships(relationships); err != nil {
		return err
	}
	providerCtx, cancel := a.providerContext(ctx)
	defer cancel()
	return normalizeProviderError(providerCtx, mutate(providerCtx, relationships))
}

func (a *authorizer) providerContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeout == 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, a.timeout)
}

func (a *authorizer) observe(ctx context.Context, observation Observation) {
	a.observer.Observe(observerContext(ctx), observation)
}

func (a *authorizer) validateObjectResult(objectType ObjectType, objects []Object) error {
	seen := make(map[string]struct{}, len(objects))
	for _, object := range objects {
		if object.Type != objectType {
			return fmt.Errorf("%w: expected object type %q, got %q", ErrInvalidProviderResponse, objectType, object.Type)
		}
		if err := a.validator.ValidateObject(object); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidProviderResponse, err)
		}
		key := object.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: duplicate object reference", ErrInvalidProviderResponse)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (a *authorizer) validateSubjectResult(subjectType ObjectType, subjects []Subject) error {
	seen := make(map[string]struct{}, len(subjects))
	for _, subject := range subjects {
		if subject.Type != subjectType {
			return fmt.Errorf("%w: expected subject type %q, got %q", ErrInvalidProviderResponse, subjectType, subject.Type)
		}
		if err := a.validator.ValidateSubject(subject); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidProviderResponse, err)
		}
		key := subject.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: duplicate subject reference", ErrInvalidProviderResponse)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateBatchProviderResult(
	request BatchCheckRequest,
	result BatchCheckResult,
	observation *Observation,
) error {
	if result.Results == nil {
		return fmt.Errorf("%w: batch result map is nil", ErrInvalidProviderResponse)
	}
	expected := make(map[string]struct{}, len(request.Checks))
	for _, item := range request.Checks {
		expected[item.CorrelationID] = struct{}{}
		outcome, exists := result.Results[item.CorrelationID]
		if !exists {
			return fmt.Errorf("%w: missing batch result", ErrInvalidProviderResponse)
		}
		if errors.Is(outcome.Err, ErrDenied) {
			return fmt.Errorf("%w: provider returned batch denial as an error", ErrInvalidProviderResponse)
		}
		if outcome.Err != nil && outcome.Decision.Allowed {
			return fmt.Errorf("%w: batch result contains both an allowed decision and an error", ErrInvalidProviderResponse)
		}
		switch {
		case outcome.Err != nil:
			observation.ErrorCount++
		case outcome.Decision.Allowed:
			observation.AllowedCount++
		default:
			observation.DeniedCount++
		}
	}
	if len(result.Results) != len(expected) {
		return fmt.Errorf("%w: unexpected batch result", ErrInvalidProviderResponse)
	}
	for correlationID := range result.Results {
		if _, exists := expected[correlationID]; !exists {
			return fmt.Errorf("%w: unknown batch correlation ID", ErrInvalidProviderResponse)
		}
	}
	return nil
}

func normalizeProviderError(ctx context.Context, err error) error {
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrProviderTimeout, err)
	}
	return err
}

func observerContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
