// Package openfga provides the OpenFGA-backed ReBAC Provider.
package openfga

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"opencsg.com/csghub-server/builder/rebac"
	commontypes "opencsg.com/csghub-server/common/types"
)

// openFGAServer contains the embedded server operations used by the Provider.
type openFGAServer interface {
	Write(context.Context, *openfgav1.WriteRequest) (*openfgav1.WriteResponse, error)
	Check(context.Context, *openfgav1.CheckRequest) (*openfgav1.CheckResponse, error)
	BatchCheck(context.Context, *openfgav1.BatchCheckRequest) (*openfgav1.BatchCheckResponse, error)
	ListObjects(context.Context, *openfgav1.ListObjectsRequest) (*openfgav1.ListObjectsResponse, error)
	ListUsers(context.Context, *openfgav1.ListUsersRequest) (*openfgav1.ListUsersResponse, error)
	Close()
}

// Provider owns an initialized in-process OpenFGA server.
type Provider struct {
	server               openFGAServer
	storeID              string
	authorizationModelID string
	defaultProvider      bool
	closeOnce            sync.Once
}

var _ rebac.Provider = (*Provider)(nil)

var (
	openfgaProvider   *Provider
	openfgaProviderMu sync.Mutex
)

// providerOptions configures a standalone OpenFGA Provider.
type providerOptions struct {
	authorizationModelID string
	pgxPool              *pgxpool.Pool
}

func defaultProviderOptions() providerOptions {
	return providerOptions{
		authorizationModelID: commontypes.OpenFgaAuthorizationModelIDLatest,
	}
}

// ProviderOption customizes a Provider created by NewCustomProvider.
type ProviderOption func(*providerOptions) error

// WithAuthorizationModelID configures the authorization model used by a custom Provider.
func WithAuthorizationModelID(authorizationModelID string) ProviderOption {
	return func(options *providerOptions) error {
		if strings.TrimSpace(authorizationModelID) == "" {
			return errors.New("OpenFGA authorization model ID is empty")
		}
		options.authorizationModelID = authorizationModelID
		return nil
	}
}

// WithPGXPool configures the application-owned PostgreSQL pool used by a custom Provider.
func WithPGXPool(pool *pgxpool.Pool) ProviderOption {
	return func(options *providerOptions) error {
		if pool == nil {
			return errors.New("pgxpool is nil")
		}
		options.pgxPool = pool
		return nil
	}
}

// NewDefaultProvider initializes the process-wide OpenFGA Provider using the application database.
// The returned Provider is cached and reused by subsequent calls.
func NewDefaultProvider() (*Provider, error) {
	openfgaProviderMu.Lock()
	defer openfgaProviderMu.Unlock()
	if openfgaProvider != nil {
		return openfgaProvider, nil
	}

	options := defaultProviderOptions()
	server, err := getServer()
	if err != nil {
		return nil, err
	}
	provider := newProvider(server, options, true)
	openfgaProvider = provider
	return provider, nil
}

// NewCustomProvider initializes a standalone OpenFGA Provider with the supplied options.
// When called without options, it delegates to NewDefaultProvider and returns the cached default Provider.
// A custom Provider created with one or more options is never stored in the process-wide cache.
func NewCustomProvider(opts ...ProviderOption) (*Provider, error) {
	if len(opts) == 0 {
		return NewDefaultProvider()
	}

	options := defaultProviderOptions()
	for _, option := range opts {
		if option == nil {
			return nil, errors.New("OpenFGA provider option is nil")
		}
		if err := option(&options); err != nil {
			return nil, err
		}
	}

	var (
		server openFGAServer
		err    error
	)
	if options.pgxPool != nil {
		server, err = newServerWithPGXPool(options.pgxPool)
	} else {
		server, err = newServer()
	}
	if err != nil {
		return nil, err
	}
	return newProvider(server, options, false), nil
}

func newProvider(server openFGAServer, options providerOptions, isDefault bool) *Provider {
	return &Provider{
		server:               server,
		storeID:              commontypes.OpenFgaStoreID,
		authorizationModelID: options.authorizationModelID,
		defaultProvider:      isDefault,
	}
}

// Name returns the stable Provider name.
func (p *Provider) Name() string {
	return "openfga"
}

// Write persists direct relationship tuples through the OpenFGA Write API.
func (p *Provider) Write(ctx context.Context, relationships []rebac.Relationship) error {
	return p.write(ctx, relationships, false)
}

// Delete removes direct relationship tuples through the OpenFGA Write API.
func (p *Provider) Delete(ctx context.Context, relationships []rebac.Relationship) error {
	return p.write(ctx, relationships, true)
}

// Close releases the embedded OpenFGA server and datastore resources.
func (p *Provider) Close() {
	if p == nil || p.server == nil {
		return
	}
	p.closeOnce.Do(func() {
		p.server.Close()
		if !p.defaultProvider {
			return
		}
		openfgaProviderMu.Lock()
		if openfgaProvider == p {
			openfgaProvider = nil
		}
		openfgaProviderMu.Unlock()
		clearCachedServer(p.server)
	})
}

// Check evaluates one authorization request through OpenFGA's Check API.
func (p *Provider) Check(ctx context.Context, request rebac.CheckRequest) (rebac.Decision, error) {
	protoRequest, err := p.checkRequest(request)
	if err != nil {
		return rebac.Decision{}, err
	}
	response, err := p.server.Check(ctx, protoRequest)
	if err != nil {
		return rebac.Decision{}, fmt.Errorf("OpenFGA Check: %w", err)
	}
	if response == nil {
		return rebac.Decision{}, fmt.Errorf("OpenFGA Check: empty response")
	}
	return rebac.Decision{Allowed: response.GetAllowed()}, nil
}

// BatchCheck evaluates independent authorization requests through OpenFGA's BatchCheck API.
func (p *Provider) BatchCheck(ctx context.Context, request rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
	checks := make([]*openfgav1.BatchCheckItem, 0, len(request.Checks))
	for _, item := range request.Checks {
		check, err := p.checkRequest(item.Check)
		if err != nil {
			return rebac.BatchCheckResult{}, err
		}
		checks = append(checks, &openfgav1.BatchCheckItem{
			TupleKey:         check.TupleKey,
			ContextualTuples: check.ContextualTuples,
			Context:          check.Context,
			CorrelationId:    item.CorrelationID,
		})
	}
	response, err := p.server.BatchCheck(ctx, &openfgav1.BatchCheckRequest{
		StoreId:              p.getStoreID(),
		AuthorizationModelId: p.getAuthorizationModelID(),
		Checks:               checks,
		Consistency:          consistency(requestConsistency(request)),
	})
	if err != nil {
		return rebac.BatchCheckResult{}, fmt.Errorf("OpenFGA BatchCheck: %w", err)
	}
	if response == nil {
		return rebac.BatchCheckResult{}, fmt.Errorf("OpenFGA BatchCheck: empty response")
	}
	if len(response.GetResult()) != len(request.Checks) {
		return rebac.BatchCheckResult{}, fmt.Errorf("OpenFGA BatchCheck: unexpected result count")
	}
	expectedCorrelationIDs := make(map[string]struct{}, len(request.Checks))
	for _, item := range request.Checks {
		expectedCorrelationIDs[item.CorrelationID] = struct{}{}
	}
	for correlationID := range response.GetResult() {
		if _, ok := expectedCorrelationIDs[correlationID]; !ok {
			return rebac.BatchCheckResult{}, fmt.Errorf("OpenFGA BatchCheck: unexpected correlation ID %q", correlationID)
		}
	}
	result := rebac.BatchCheckResult{Results: make(map[string]rebac.BatchCheckOutcome, len(request.Checks))}
	for _, item := range request.Checks {
		protoResult, ok := response.GetResult()[item.CorrelationID]
		if !ok || protoResult == nil {
			return rebac.BatchCheckResult{}, fmt.Errorf("OpenFGA BatchCheck: missing result for correlation ID %q", item.CorrelationID)
		}
		if checkError := protoResult.GetError(); checkError != nil {
			result.Results[item.CorrelationID] = rebac.BatchCheckOutcome{Err: fmt.Errorf("OpenFGA BatchCheck: %s", checkError.GetMessage())}
			continue
		}
		result.Results[item.CorrelationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: protoResult.GetAllowed()}}
	}
	return result, nil
}

// ListObjects lists authorized objects through OpenFGA's ListObjects API.
func (p *Provider) ListObjects(ctx context.Context, request rebac.ListObjectsRequest) (rebac.ListObjectsResult, error) {
	conditionContext, err := conditionContext(request.ConditionContext)
	if err != nil {
		return rebac.ListObjectsResult{}, err
	}
	response, err := p.server.ListObjects(ctx, &openfgav1.ListObjectsRequest{
		StoreId:              p.getStoreID(),
		AuthorizationModelId: p.getAuthorizationModelID(),
		Type:                 string(request.ObjectType),
		Relation:             request.Relation.String(),
		User:                 request.Subject.String(),
		ContextualTuples:     contextualTuples(request.ContextualRelationships),
		Context:              conditionContext,
		Consistency:          consistency(request.Consistency),
	})
	if err != nil {
		return rebac.ListObjectsResult{}, fmt.Errorf("OpenFGA ListObjects: %w", err)
	}
	if response == nil {
		return rebac.ListObjectsResult{}, fmt.Errorf("OpenFGA ListObjects: empty response")
	}
	objects := make([]rebac.Object, 0, len(response.GetObjects()))
	for _, object := range response.GetObjects() {
		parsed, err := parseObject(object)
		if err != nil {
			return rebac.ListObjectsResult{}, err
		}
		objects = append(objects, parsed)
	}
	return rebac.ListObjectsResult{Objects: objects}, nil
}

// ListSubjects lists authorized subjects through OpenFGA's ListUsers API.
func (p *Provider) ListSubjects(ctx context.Context, request rebac.ListSubjectsRequest) (rebac.ListSubjectsResult, error) {
	conditionContext, err := conditionContext(request.ConditionContext)
	if err != nil {
		return rebac.ListSubjectsResult{}, err
	}
	response, err := p.server.ListUsers(ctx, &openfgav1.ListUsersRequest{
		StoreId:              p.getStoreID(),
		AuthorizationModelId: p.getAuthorizationModelID(),
		Object:               &openfgav1.Object{Type: string(request.Object.Type), Id: request.Object.ID},
		Relation:             request.Relation.String(),
		UserFilters:          []*openfgav1.UserTypeFilter{{Type: string(request.SubjectType)}},
		ContextualTuples:     tupleKeys(request.ContextualRelationships),
		Context:              conditionContext,
		Consistency:          consistency(request.Consistency),
	})
	if err != nil {
		return rebac.ListSubjectsResult{}, fmt.Errorf("OpenFGA ListUsers: %w", err)
	}
	if response == nil {
		return rebac.ListSubjectsResult{}, fmt.Errorf("OpenFGA ListUsers: empty response")
	}
	subjects := make([]rebac.Subject, 0, len(response.GetUsers()))
	for _, user := range response.GetUsers() {
		subject, err := parseUser(user)
		if err != nil {
			return rebac.ListSubjectsResult{}, err
		}
		subjects = append(subjects, subject)
	}
	return rebac.ListSubjectsResult{Subjects: subjects}, nil
}

func (p *Provider) getStoreID() string {
	if p.storeID != "" {
		return p.storeID
	}
	return commontypes.OpenFgaStoreID
}

func (p *Provider) getAuthorizationModelID() string {
	if p.authorizationModelID != "" {
		return p.authorizationModelID
	}
	return commontypes.OpenFgaAuthorizationModelIDLatest
}

// checkRequest converts the public check contract into an OpenFGA request.
func (p *Provider) checkRequest(request rebac.CheckRequest) (*openfgav1.CheckRequest, error) {
	conditionContext, err := conditionContext(request.ConditionContext)
	if err != nil {
		return nil, err
	}
	return &openfgav1.CheckRequest{
		StoreId:              p.getStoreID(),
		AuthorizationModelId: p.getAuthorizationModelID(),
		TupleKey:             &openfgav1.CheckRequestTupleKey{User: request.Subject.String(), Relation: request.Relation.String(), Object: request.Object.String()},
		ContextualTuples:     contextualTuples(request.ContextualRelationships),
		Context:              conditionContext,
		Consistency:          consistency(request.Consistency),
	}, nil
}

// contextualTuples converts contextual relationships into OpenFGA's wrapper type.
func contextualTuples(relationships []rebac.Relationship) *openfgav1.ContextualTupleKeys {
	if len(relationships) == 0 {
		return nil
	}
	return &openfgav1.ContextualTupleKeys{TupleKeys: tupleKeys(relationships)}
}

// tupleKeys converts public relationships into OpenFGA tuple keys.
func tupleKeys(relationships []rebac.Relationship) []*openfgav1.TupleKey {
	keys := make([]*openfgav1.TupleKey, 0, len(relationships))
	for _, relationship := range relationships {
		keys = append(keys, &openfgav1.TupleKey{User: relationship.Subject.String(), Relation: string(relationship.Relation), Object: relationship.Object.String()})
	}
	return keys
}

// conditionContext converts condition data to protobuf Struct values.
func conditionContext(values map[string]any) (*structpb.Struct, error) {
	if values == nil {
		return nil, nil
	}
	converted, err := structpb.NewStruct(values)
	if err != nil {
		return nil, fmt.Errorf("OpenFGA request context: %w", err)
	}
	return converted, nil
}

// consistency maps the public consistency enum to OpenFGA's enum.
func consistency(value rebac.Consistency) openfgav1.ConsistencyPreference {
	switch value {
	case rebac.ConsistencyMinimizeLatency:
		return openfgav1.ConsistencyPreference_MINIMIZE_LATENCY
	case rebac.ConsistencyHigher:
		return openfgav1.ConsistencyPreference_HIGHER_CONSISTENCY
	default:
		return openfgav1.ConsistencyPreference_UNSPECIFIED
	}
}

// requestConsistency selects the consistency used by OpenFGA's batch API.
func requestConsistency(request rebac.BatchCheckRequest) rebac.Consistency {
	selected := rebac.ConsistencyDefault
	for _, item := range request.Checks {
		if item.Check.Consistency > selected {
			selected = item.Check.Consistency
		}
	}
	return selected
}

// parseObject converts OpenFGA's type:id object reference to a public object.
func parseObject(value string) (rebac.Object, error) {
	typeName, id, ok := strings.Cut(value, ":")
	if !ok || typeName == "" || id == "" {
		return rebac.Object{}, fmt.Errorf("OpenFGA response contains invalid object %q", value)
	}
	return rebac.NewObject(rebac.ObjectType(typeName), id), nil
}

// parseUser converts an OpenFGA user, userset, or wildcard into a public subject.
func parseUser(user *openfgav1.User) (rebac.Subject, error) {
	if user == nil {
		return rebac.Subject{}, fmt.Errorf("OpenFGA response contains nil user")
	}
	if object := user.GetObject(); object != nil {
		return rebac.NewSubject(rebac.ObjectType(object.GetType()), object.GetId()), nil
	}
	if userset := user.GetUserset(); userset != nil {
		return rebac.NewUserset(rebac.ObjectType(userset.GetType()), userset.GetId(), rebac.Relation(userset.GetRelation())), nil
	}
	if wildcard := user.GetWildcard(); wildcard != nil {
		return rebac.NewWildcardSubject(rebac.ObjectType(wildcard.GetType())), nil
	}
	return rebac.Subject{}, fmt.Errorf("OpenFGA response contains empty user")
}

func (p *Provider) write(ctx context.Context, relationships []rebac.Relationship, deleting bool) error {
	request := &openfgav1.WriteRequest{
		StoreId:              p.getStoreID(),
		AuthorizationModelId: p.getAuthorizationModelID(),
	}
	if deleting {
		tupleKeys := make([]*openfgav1.TupleKeyWithoutCondition, 0, len(relationships))
		for _, relationship := range relationships {
			tupleKeys = append(tupleKeys, &openfgav1.TupleKeyWithoutCondition{
				User:     relationship.Subject.String(),
				Relation: string(relationship.Relation),
				Object:   relationship.Object.String(),
			})
		}
		request.Deletes = &openfgav1.WriteRequestDeletes{TupleKeys: tupleKeys}
	} else {
		tupleKeys := make([]*openfgav1.TupleKey, 0, len(relationships))
		for _, relationship := range relationships {
			tupleKeys = append(tupleKeys, &openfgav1.TupleKey{
				User:     relationship.Subject.String(),
				Relation: string(relationship.Relation),
				Object:   relationship.Object.String(),
			})
		}
		request.Writes = &openfgav1.WriteRequestWrites{TupleKeys: tupleKeys}
	}

	if _, err := p.server.Write(ctx, request); err != nil {
		return fmt.Errorf("OpenFGA Write: %w", err)
	}
	return nil
}
