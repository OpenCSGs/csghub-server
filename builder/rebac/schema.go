package rebac

import (
	"fmt"
	"strings"
)

// RelationKind distinguishes storable direct relationships from computed permissions.
type RelationKind uint8

const (
	// RelationKindDirect identifies a relationship that can be stored as a tuple.
	RelationKindDirect RelationKind = iota + 1
	// RelationKindComputed identifies a computed relationship exposed by the model.
	RelationKindComputed
	// RelationKindPermission identifies a computed permission used by authorization checks.
	RelationKindPermission
)

// ObjectTypeDefinition declares the relationships and permissions supported by an object type.
type ObjectTypeDefinition struct {
	Type              ObjectType
	DirectRelations   []Relation
	ComputedRelations []Relation
	Permissions       []Permission
}

// Schema is an immutable registry of allowed object types and relations.
type Schema struct {
	relations   map[ObjectType]map[Relation]RelationKind
	definitions []ObjectTypeDefinition
}

// NewSchema validates and creates an immutable schema registry.
func NewSchema(definitions ...ObjectTypeDefinition) (*Schema, error) {
	schema := &Schema{
		relations:   make(map[ObjectType]map[Relation]RelationKind, len(definitions)),
		definitions: make([]ObjectTypeDefinition, 0, len(definitions)),
	}

	for _, definition := range definitions {
		if !validModelName(string(definition.Type)) {
			return nil, fmt.Errorf("%w: invalid object type %q", ErrInvalidSchema, definition.Type)
		}
		if isReservedModelName(string(definition.Type)) {
			return nil, fmt.Errorf("%w: object type %q is reserved", ErrInvalidSchema, definition.Type)
		}
		if _, exists := schema.relations[definition.Type]; exists {
			return nil, fmt.Errorf("%w: duplicate object type %q", ErrInvalidSchema, definition.Type)
		}

		relations := make(map[Relation]RelationKind, len(definition.DirectRelations)+len(definition.ComputedRelations)+len(definition.Permissions))
		if err := addSchemaRelations(relations, definition.Type, definition.DirectRelations, RelationKindDirect); err != nil {
			return nil, err
		}
		if err := addSchemaRelations(relations, definition.Type, definition.ComputedRelations, RelationKindComputed); err != nil {
			return nil, err
		}
		if err := addSchemaRelations(relations, definition.Type, definition.Permissions, RelationKindPermission); err != nil {
			return nil, err
		}

		schema.relations[definition.Type] = relations
		schema.definitions = append(schema.definitions, cloneObjectTypeDefinition(definition))
	}

	return schema, nil
}

// DefaultSchema creates the common schema and merges edition-specific definitions.
func DefaultSchema() (*Schema, error) {
	return NewSchema(CommonSchemaDefinitions()...)
}

// CommonSchemaDefinitions returns the initial schema shared by CE, EE, and SaaS editions.
func CommonSchemaDefinitions() []ObjectTypeDefinition {
	return []ObjectTypeDefinition{
		{
			Type:            ObjectTypeUser,
			DirectRelations: []Relation{RelationOwner},
			Permissions:     []Permission{PermissionCanRead, UserCanWrite},
		},
		{
			Type: ObjectTypeOrganization,
			DirectRelations: []Relation{
				RelationAdmin,
				RelationWriter,
				RelationReader,
				RelationParent,
				RelationChild,
			},
			ComputedRelations: []Relation{RelationMember, RelationMemberFromChild},
			Permissions: []Permission{
				OrganizationCanRead,
				OrganizationCanWrite,
				OrganizationCanAdmin,
			},
		},
		{
			Type: ObjectTypeNamespace,
			DirectRelations: []Relation{
				RelationOwner,
				RelationOrganization,
				RelationAdmin,
				RelationWriter,
				RelationReader,
			},
			Permissions: []Permission{NamespaceCanRead, NamespaceCanWrite, NamespaceCanAdmin},
		},
		{
			Type: ObjectTypeRepository,
			DirectRelations: []Relation{
				RelationOwner,
				RelationOrganization,
				RelationOrganizationDirect,
				RelationAdmin,
				RelationReader,
				RelationWriter,
			},
			Permissions: []Permission{RepositoryCanRead, RepositoryCanWrite, RepositoryCanAdmin},
		},
	}
}

// HasObjectType reports whether an object type is registered.
func (s *Schema) HasObjectType(objectType ObjectType) bool {
	if s == nil {
		return false
	}
	_, ok := s.relations[objectType]
	return ok
}

// RelationKind returns the kind of a registered relation.
func (s *Schema) RelationKind(objectType ObjectType, relation Relation) (RelationKind, bool) {
	if s == nil {
		return 0, false
	}
	relations, ok := s.relations[objectType]
	if !ok {
		return 0, false
	}
	kind, ok := relations[relation]
	return kind, ok
}

// ValidateRelation validates that a relation is registered for an object type.
func (s *Schema) ValidateRelation(objectType ObjectType, relation Relation) error {
	if !s.HasObjectType(objectType) {
		return fmt.Errorf("%w: %q", ErrUnsupportedObjectType, objectType)
	}
	if _, ok := s.RelationKind(objectType, relation); !ok {
		return fmt.Errorf("%w: %q on %q", ErrUnsupportedRelation, relation, objectType)
	}
	return nil
}

// ValidateCheckRelation validates that a query relation is registered with the matching schema kind.
func (s *Schema) ValidateCheckRelation(objectType ObjectType, relation CheckRelation) error {
	if relation == nil {
		return fmt.Errorf("%w: query relation is nil", ErrInvalidRelation)
	}
	relationName := Relation(relation.String())
	if err := s.ValidateRelation(objectType, relationName); err != nil {
		return err
	}
	kind, _ := s.RelationKind(objectType, relationName)
	switch relation.(type) {
	case Relation:
		if kind != RelationKindDirect && kind != RelationKindComputed {
			return fmt.Errorf("%w: %q on %q is not a relation", ErrInvalidRelation, relationName, objectType)
		}
	case Permission:
		if kind != RelationKindPermission {
			return fmt.Errorf("%w: %q on %q is a direct relation", ErrInvalidRelation, relationName, objectType)
		}
	default:
		return fmt.Errorf("%w: unsupported query relation type", ErrInvalidRelation)
	}
	return nil
}

// ValidateDirectRelation validates that a relation can be stored as a direct relationship.
func (s *Schema) ValidateDirectRelation(objectType ObjectType, relation Relation) error {
	if err := s.ValidateRelation(objectType, relation); err != nil {
		return err
	}
	kind, _ := s.RelationKind(objectType, relation)
	if kind != RelationKindDirect {
		return fmt.Errorf("%w: %q on %q is not a direct relation", ErrInvalidRelation, relation, objectType)
	}
	return nil
}

// Definitions returns a defensive copy of the registered schema definitions.
func (s *Schema) Definitions() []ObjectTypeDefinition {
	if s == nil {
		return nil
	}
	definitions := make([]ObjectTypeDefinition, 0, len(s.definitions))
	for _, definition := range s.definitions {
		definitions = append(definitions, cloneObjectTypeDefinition(definition))
	}
	return definitions
}

func addSchemaRelations[T ~string](
	registered map[Relation]RelationKind,
	objectType ObjectType,
	relations []T,
	kind RelationKind,
) error {
	for _, value := range relations {
		relation := Relation(value)
		if !validModelName(string(relation)) {
			return fmt.Errorf("%w: invalid relation %q on %q", ErrInvalidSchema, relation, objectType)
		}
		if isReservedModelName(string(relation)) {
			return fmt.Errorf("%w: relation %q on %q is reserved by OpenFGA", ErrInvalidSchema, relation, objectType)
		}
		if kind == RelationKindDirect && strings.HasPrefix(string(relation), "can_") {
			return fmt.Errorf("%w: direct relation %q must not use the can_ prefix", ErrInvalidSchema, relation)
		}
		if kind == RelationKindPermission && !strings.HasPrefix(string(relation), "can_") {
			return fmt.Errorf("%w: permission %q must use the can_ prefix", ErrInvalidSchema, relation)
		}
		if kind == RelationKindComputed && strings.HasPrefix(string(relation), "can_") {
			return fmt.Errorf("%w: computed relation %q must not use the can_ prefix", ErrInvalidSchema, relation)
		}
		if _, exists := registered[relation]; exists {
			return fmt.Errorf("%w: duplicate relation %q on %q", ErrInvalidSchema, relation, objectType)
		}
		registered[relation] = kind
	}
	return nil
}

func cloneObjectTypeDefinition(definition ObjectTypeDefinition) ObjectTypeDefinition {
	return ObjectTypeDefinition{
		Type:              definition.Type,
		DirectRelations:   append([]Relation(nil), definition.DirectRelations...),
		ComputedRelations: append([]Relation(nil), definition.ComputedRelations...),
		Permissions:       append([]Permission(nil), definition.Permissions...),
	}
}

func isReservedModelName(value string) bool {
	return value == "self" || value == "this"
}
