package rebac

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// DefaultMaxBatchSize matches the default OpenFGA BatchCheck request limit.
	DefaultMaxBatchSize = 50
	// DefaultMaxContextualRelationships limits contextual relationships in one request.
	DefaultMaxContextualRelationships = 100
	// DefaultMaxConditionContextBytes limits the serialized condition context size.
	DefaultMaxConditionContextBytes = 64 * 1024
	// DefaultMaxConditionContextDepth limits nesting depth for condition context values.
	DefaultMaxConditionContextDepth = 16
	// DefaultMaxConditionContextValues limits the total number of condition context values.
	DefaultMaxConditionContextValues = 1024
	// DefaultMaxIdentifierBytes limits the length of one object or subject identifier.
	DefaultMaxIdentifierBytes = 512
	// DefaultMaxCorrelationIDBytes limits the length of one batch correlation ID.
	DefaultMaxCorrelationIDBytes = 36
)

var modelNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// correlationIDPattern matches the OpenFGA BatchCheck correlation ID contract.
var correlationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,36}$`)

// Validator validates requests against an authorization schema independently of a Provider.
type Validator struct {
	schema *Schema
}

// NewValidator creates a schema-driven request validator.
func NewValidator(schema *Schema) (*Validator, error) {
	if schema == nil {
		return nil, fmt.Errorf("%w: schema is nil", ErrInvalidSchema)
	}
	return &Validator{schema: schema}, nil
}

// ValidateCheckRequest validates one authorization request.
func (v *Validator) ValidateCheckRequest(request CheckRequest) error {
	if err := v.ValidateSubject(request.Subject); err != nil {
		return err
	}
	if err := v.ValidateObject(request.Object); err != nil {
		return err
	}
	if err := validateCheckRelationSyntax(request.Relation); err != nil {
		return err
	}
	if err := v.schema.ValidateCheckRelation(request.Object.Type, request.Relation); err != nil {
		return err
	}
	if err := validateConsistency(request.Consistency); err != nil {
		return err
	}
	if err := validateConditionContext(request.ConditionContext); err != nil {
		return err
	}
	return v.validateContextualRelationships(request.ContextualRelationships)
}

// ValidateBatchCheckRequest validates correlation IDs and every check in a batch.
func (v *Validator) ValidateBatchCheckRequest(request BatchCheckRequest, maxBatchSize int) error {
	if len(request.Checks) == 0 {
		return fmt.Errorf("%w: batch is empty", ErrInvalidRequest)
	}
	if maxBatchSize <= 0 {
		return fmt.Errorf("%w: maximum batch size must be positive", ErrInvalidRequest)
	}
	if len(request.Checks) > maxBatchSize {
		return fmt.Errorf("%w: got %d checks, maximum is %d", ErrBatchLimitExceeded, len(request.Checks), maxBatchSize)
	}

	seen := make(map[string]struct{}, len(request.Checks))
	for index, item := range request.Checks {
		if err := validateCorrelationID(item.CorrelationID); err != nil {
			return fmt.Errorf("%w: check %d: %w", ErrInvalidRequest, index, err)
		}
		if _, exists := seen[item.CorrelationID]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateCorrelationID, item.CorrelationID)
		}
		seen[item.CorrelationID] = struct{}{}
		if err := v.ValidateCheckRequest(item.Check); err != nil {
			return fmt.Errorf("%w: check %q: %w", ErrInvalidRequest, item.CorrelationID, err)
		}
	}
	return nil
}

// ValidateListObjectsRequest validates an object listing request with authorization constraints.
func (v *Validator) ValidateListObjectsRequest(request ListObjectsRequest) error {
	if err := v.ValidateSubject(request.Subject); err != nil {
		return err
	}
	if !v.schema.HasObjectType(request.ObjectType) {
		return fmt.Errorf("%w: %q", ErrUnsupportedObjectType, request.ObjectType)
	}
	if err := validateCheckRelationSyntax(request.Relation); err != nil {
		return err
	}
	if err := v.schema.ValidateCheckRelation(request.ObjectType, request.Relation); err != nil {
		return err
	}
	if err := validateConsistency(request.Consistency); err != nil {
		return err
	}
	if err := validateConditionContext(request.ConditionContext); err != nil {
		return err
	}
	return v.validateContextualRelationships(request.ContextualRelationships)
}

// ValidateListSubjectsRequest validates a subject listing request with authorization constraints.
func (v *Validator) ValidateListSubjectsRequest(request ListSubjectsRequest) error {
	if err := v.ValidateObject(request.Object); err != nil {
		return err
	}
	if !v.schema.HasObjectType(request.SubjectType) {
		return fmt.Errorf("%w: subject type %q", ErrUnsupportedObjectType, request.SubjectType)
	}
	if err := validateCheckRelationSyntax(request.Relation); err != nil {
		return err
	}
	if err := v.schema.ValidateCheckRelation(request.Object.Type, request.Relation); err != nil {
		return err
	}
	if err := validateConsistency(request.Consistency); err != nil {
		return err
	}
	if err := validateConditionContext(request.ConditionContext); err != nil {
		return err
	}
	return v.validateContextualRelationships(request.ContextualRelationships)
}

// ValidateSubject validates a concrete subject, wildcard, or userset.
func (v *Validator) ValidateSubject(subject Subject) error {
	if !v.schema.HasObjectType(subject.Type) {
		return fmt.Errorf("%w: subject type %q", ErrUnsupportedObjectType, subject.Type)
	}
	if err := validateIdentifier(subject.ID, true); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSubject, err)
	}
	if subject.IsWildcard() && subject.IsUserset() {
		return fmt.Errorf("%w: wildcard cannot have a userset relation", ErrInvalidSubject)
	}
	if subject.Relation != "" {
		if err := validateRelationSyntax(subject.Relation); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSubject, err)
		}
		if err := v.schema.ValidateRelation(subject.Type, subject.Relation); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSubject, err)
		}
	}
	return nil
}

// ValidateObject validates a protected object reference.
func (v *Validator) ValidateObject(object Object) error {
	if !v.schema.HasObjectType(object.Type) {
		return fmt.Errorf("%w: %q", ErrUnsupportedObjectType, object.Type)
	}
	if err := validateIdentifier(object.ID, false); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidObject, err)
	}
	return nil
}

// ValidateRelationship validates an OpenFGA-compatible direct relationship.
func (v *Validator) ValidateRelationship(relationship Relationship) error {
	if err := v.ValidateSubject(relationship.Subject); err != nil {
		return err
	}
	if err := v.ValidateObject(relationship.Object); err != nil {
		return err
	}
	if err := validateRelationSyntax(relationship.Relation); err != nil {
		return err
	}
	return v.schema.ValidateDirectRelation(relationship.Object.Type, relationship.Relation)
}

// ValidateRelationships validates a non-empty batch of direct relationship tuples.
func (v *Validator) ValidateRelationships(relationships []Relationship) error {
	if len(relationships) == 0 {
		return fmt.Errorf("%w: relationships are empty", ErrInvalidRequest)
	}
	for index, relationship := range relationships {
		if err := v.ValidateRelationship(relationship); err != nil {
			return fmt.Errorf("%w: relationship %d: %w", ErrInvalidRequest, index, err)
		}
	}
	return nil
}

func (v *Validator) validateContextualRelationships(relationships []Relationship) error {
	if len(relationships) > DefaultMaxContextualRelationships {
		return fmt.Errorf(
			"%w: got %d contextual relationships, maximum is %d",
			ErrInvalidRequest,
			len(relationships),
			DefaultMaxContextualRelationships,
		)
	}
	for index, relationship := range relationships {
		if err := v.ValidateRelationship(relationship); err != nil {
			return fmt.Errorf("%w: contextual relationship %d: %w", ErrInvalidRequest, index, err)
		}
	}
	return nil
}

func validModelName(value string) bool {
	return modelNamePattern.MatchString(value)
}

// validateCheckRelationSyntax validates the model name carried by a query relation.
func validateCheckRelationSyntax(relation CheckRelation) error {
	if relation == nil {
		return fmt.Errorf("%w: query relation is nil", ErrInvalidRelation)
	}
	return validateRelationSyntax(Relation(relation.String()))
}

func validateRelationSyntax(relation Relation) error {
	if !validModelName(string(relation)) || isReservedModelName(string(relation)) {
		return fmt.Errorf("%w: %q", ErrInvalidRelation, relation)
	}
	return nil
}

func validateIdentifier(id string, allowWildcard bool) error {
	if id == "" {
		return fmt.Errorf("identifier is empty")
	}
	if len(id) > DefaultMaxIdentifierBytes {
		return fmt.Errorf("identifier exceeds %d bytes", DefaultMaxIdentifierBytes)
	}
	if !utf8.ValidString(id) {
		return fmt.Errorf("identifier is not valid UTF-8")
	}
	if id == WildcardSubjectID {
		if allowWildcard {
			return nil
		}
		return fmt.Errorf("wildcard is not a valid object identifier")
	}
	for _, value := range id {
		if value == ':' || value == '#' || unicode.IsControl(value) || unicode.IsSpace(value) {
			return fmt.Errorf("identifier contains a reserved character")
		}
	}
	return nil
}

func validateCorrelationID(id string) error {
	if !correlationIDPattern.MatchString(id) {
		return fmt.Errorf("correlation ID must match pattern %q", correlationIDPattern.String())
	}
	return nil
}

func validateConsistency(consistency Consistency) error {
	if !consistency.Valid() {
		return fmt.Errorf("%w: unknown consistency value %d", ErrInvalidRequest, consistency)
	}
	return nil
}

func validateConditionContext(context map[string]any) error {
	if context == nil {
		return nil
	}
	valueCount := 0
	if err := validateJSONValue(reflect.ValueOf(context), 0, &valueCount); err != nil {
		return fmt.Errorf("%w: invalid condition context: %v", ErrInvalidRequest, err)
	}
	encoded, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("%w: invalid condition context: %v", ErrInvalidRequest, err)
	}
	if len(encoded) > DefaultMaxConditionContextBytes {
		return fmt.Errorf(
			"%w: condition context exceeds %d bytes",
			ErrInvalidRequest,
			DefaultMaxConditionContextBytes,
		)
	}
	return nil
}

func validateJSONValue(value reflect.Value, depth int, count *int) error {
	if depth > DefaultMaxConditionContextDepth {
		return fmt.Errorf("maximum depth %d exceeded", DefaultMaxConditionContextDepth)
	}
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		return validateJSONValue(value.Elem(), depth, count)
	}

	*count++
	if *count > DefaultMaxConditionContextValues {
		return fmt.Errorf("maximum value count %d exceeded", DefaultMaxConditionContextValues)
	}

	switch value.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return nil
	case reflect.Float32, reflect.Float64:
		floating := value.Float()
		if math.IsNaN(floating) || math.IsInf(floating, 0) {
			return fmt.Errorf("non-finite numbers are not JSON-compatible")
		}
		return nil
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("map keys must be strings")
		}
		iterator := value.MapRange()
		for iterator.Next() {
			if err := validateJSONValue(iterator.Value(), depth+1, count); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array, reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			if err := validateJSONValue(value.Index(index), depth+1, count); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s values are not JSON-compatible", strings.ToLower(value.Kind().String()))
	}
}
