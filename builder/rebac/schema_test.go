package rebac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultSchemaContainsCommonTypes(t *testing.T) {
	schema, err := DefaultSchema()
	require.NoError(t, err)

	for _, objectType := range []ObjectType{
		ObjectTypeUser,
		ObjectTypeOrganization,
		ObjectTypeNamespace,
		ObjectTypeRepository,
		ObjectTypeKnowledgeBase,
	} {
		require.True(t, schema.HasObjectType(objectType), objectType)
	}
	require.NoError(t, schema.ValidateDirectRelation(ObjectTypeRepository, RelationReader))
	require.NoError(t, schema.ValidateDirectRelation(ObjectTypeRepository, RelationOrganization))
	require.NoError(t, schema.ValidateDirectRelation(ObjectTypeRepository, RelationOrganizationDirect))
	require.NoError(t, schema.ValidateDirectRelation(ObjectTypeNamespace, RelationOrganization))
	require.NoError(t, schema.ValidateDirectRelation(ObjectTypeOrganization, RelationChild))
	require.NoError(t, schema.ValidateRelation(ObjectTypeOrganization, RelationMember))
	require.NoError(t, schema.ValidateCheckRelation(ObjectTypeOrganization, RelationMember))
	require.ErrorIs(t, schema.ValidateDirectRelation(ObjectTypeOrganization, RelationMember), ErrInvalidRelation)
	require.NoError(t, schema.ValidateRelation(ObjectTypeOrganization, RelationMemberFromChild))
	require.NoError(t, schema.ValidateCheckRelation(ObjectTypeOrganization, RelationMemberFromChild))
	require.ErrorIs(t, schema.ValidateDirectRelation(ObjectTypeOrganization, RelationMemberFromChild), ErrInvalidRelation)
	require.NoError(t, schema.ValidateCheckRelation(ObjectTypeUser, PermissionCanRead))
	require.NoError(t, schema.ValidateCheckRelation(ObjectTypeRepository, RepositoryCanRead))
	require.NoError(t, schema.ValidateDirectRelation(ObjectTypeKnowledgeBase, RelationNamespace))
	require.NoError(t, schema.ValidateDirectRelation(ObjectTypeKnowledgeBase, RelationPublic))
	require.NoError(t, schema.ValidateCheckRelation(ObjectTypeKnowledgeBase, KnowledgeBaseCanRead))
	require.NoError(t, schema.ValidateCheckRelation(ObjectTypeKnowledgeBase, KnowledgeBaseCanWrite))
	require.NoError(t, schema.ValidateCheckRelation(ObjectTypeKnowledgeBase, KnowledgeBaseCanAdmin))
	require.ErrorIs(t, schema.ValidateDirectRelation(ObjectTypeRepository, Relation(RepositoryCanRead)), ErrInvalidRelation)
	require.ErrorIs(t, schema.ValidateRelation(ObjectTypeRepository, RelationMember), ErrUnsupportedRelation)
	require.ErrorIs(t, schema.ValidateCheckRelation("unknown", RepositoryCanRead), ErrUnsupportedObjectType)
}

func TestNewSchemaRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		definitions []ObjectTypeDefinition
	}{
		{
			name: "invalid object type",
			definitions: []ObjectTypeDefinition{{
				Type: "Invalid-Type",
			}},
		},
		{
			name: "OpenFGA reserved self relation",
			definitions: []ObjectTypeDefinition{{
				Type:            "document",
				DirectRelations: []Relation{"self"},
			}},
		},
		{
			name: "duplicate relation",
			definitions: []ObjectTypeDefinition{{
				Type:            "document",
				DirectRelations: []Relation{RelationOwner, RelationOwner},
			}},
		},
		{
			name: "permission without prefix",
			definitions: []ObjectTypeDefinition{{
				Type:        "document",
				Permissions: []Permission{"read"},
			}},
		},
		{
			name: "direct relation with permission prefix",
			definitions: []ObjectTypeDefinition{{
				Type:            "document",
				DirectRelations: []Relation{Relation(PermissionCanRead)},
			}},
		},
		{
			name: "computed relation with permission prefix",
			definitions: []ObjectTypeDefinition{{
				Type:              "document",
				ComputedRelations: []Relation{Relation(PermissionCanRead)},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSchema(test.definitions...)
			require.ErrorIs(t, err, ErrInvalidSchema)
		})
	}
}

func TestSchemaDefinitionsAreDefensiveCopies(t *testing.T) {
	definition := ObjectTypeDefinition{
		Type:              "document",
		DirectRelations:   []Relation{RelationOwner},
		ComputedRelations: []Relation{RelationMember},
		Permissions:       []Permission{PermissionCanRead},
	}
	schema, err := NewSchema(definition)
	require.NoError(t, err)

	definition.DirectRelations[0] = RelationAdmin
	copyOne := schema.Definitions()
	copyOne[0].DirectRelations[0] = RelationMember
	copyOne[0].ComputedRelations[0] = RelationAdmin
	copyTwo := schema.Definitions()

	require.Equal(t, RelationOwner, copyTwo[0].DirectRelations[0])
	require.Equal(t, RelationMember, copyTwo[0].ComputedRelations[0])
}
