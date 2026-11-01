# How to Use ReBAC

Before you start, check which situation applies to you:

1. **You need to change or update the model**: For example, you want to add a resource type, add a relation, or change permission rules. Create a new model version, run its migration, and update Latest when everything is ready. See sections 1–3 and section 6.
2. **You only need to use the existing model**: For example, you want to grant or revoke access, check permissions, or list accessible resources. Initialize ReBAC and call its methods. **You do not need to modify the model, create a new model version, or update Latest.** See sections 4–5. This assumes the existing model has already been initialized in the database; the first deployment still needs the initialization migrations in section 3.

Adding or deleting relationships for a user means using the model, rather than updating it. You only need to change the model when its existing rules cannot express the permissions you need.

## 1. Changing the Model

Model files are in `builder/store/database/openfga`:

- `dsl_v1_0.go`: Permissions for users, organizations, namespaces, and repositories.
- `dsl_v1_1.go`: Adds `organization.member_from_child` to query membership that includes child organizations.
- `dsl_v1_2.go`: Adds knowledge base permissions. This is the current default version.

To change permission rules, copy the current model into a new version. For example, copy `dsl_v1_2.go` to `dsl_v1_3.go`, rename the constant to `AuthorizationModelVer1_3`, and then edit the DSL inside it. Keep previously used models instead of overwriting them.

Model IDs are defined in [`common/types/rebac.go`](../../common/types/rebac.go). Add a new ID for the new model, such as `OpenFgaAuthorizationModelIDVer1_3`, and generate its value with `ulid.Make().String()`. Once the new model's migration has run, point `OpenFgaAuthorizationModelIDLatest` to it and redeploy the service.

Versions 1.0, 1.1, and 1.2 here are the project's own model versions. The `schema 1.1` declaration at the start of the model is the OpenFGA language version. It does not need to change when you add a project model version.

If you add object types, relations, or permissions, also update `object_type.go`, `permission.go`, and `schema.go` under `builder/rebac`. Otherwise, ReBAC will reject the new types or relations as unsupported parameters.

## 2. Writing a Model

Start with this simplified repository model:

```text
model
  schema 1.1

type user

type repository
  relations
    define owner: [user]
    define writer: [user]
    define reader: [user]

    define can_write: owner or writer
    define can_read: can_write or reader
```

This model means:

- `owner`, `writer`, and `reader` are relations you can write. `[user]` means the subject of the relation is a user.
- `can_write` means both owners and writers can write.
- `can_read` means anyone who can write, as well as readers, can read.

After you write a `reader` relationship for a user, checking `can_read` will allow access. `can_read` and `can_write` are computed permissions; do not write them as relationships.

The project's full model also includes permissions inherited from organizations. For example, `can_read from organization` follows the repository's `organization` relationship and checks read permission on the associated organization. Keep the other definitions in the full model when making changes; the example above only explains the syntax.

For the syntax, see OpenFGA's [configuration language documentation](https://openfga.dev/docs/configuration-language) and [modeling introduction](https://openfga.dev/docs/modeling/getting-started). For model versioning, see [immutable models](https://openfga.dev/docs/getting-started/immutable-models).

## 3. Saving the Model to the Database

After writing the model file, you also need a migration to save it to the database. Use this file as a reference:

[`20260827160000_update_openfga_store_model.go`](../store/database/migrations/20260827160000_update_openfga_store_model.go)

First, generate a new migration (or manually copy `20260827160000_update_openfga_store_model.go`):

```bash
go run -tags saas cmd/csghub-server/main.go --config config.toml migration create_go update_openfga_model
```

The main code in the reference file is shown below. For a new version, replace both version 1.1 constants with the corresponding constants for the new version. You can also copy the imports from the original file:

```go
// init registers model version 1.1 and its rollback.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return insertAuthorizationModel(
			ctx,
			db,
			types.OpenFgaAuthorizationModelIDVer1_1,
			modelopenfga.AuthorizationModelVer1_1,
		)
	}, func(ctx context.Context, db *bun.DB) error {
		return deleteAuthorizationModel(ctx, db, types.OpenFgaAuthorizationModelIDVer1_1)
	})
}
```

`insertAuthorizationModel` converts the DSL into an OpenFGA model and saves it to the database. `deleteAuthorizationModel` rolls back this model. Use a new ID for each new version; reusing an old ID causes an error.

For the first database setup, run `init` followed by `migrate`. For later model additions, just run `migrate`. Replace `config.toml` with your configuration file and configure PostgreSQL and Redis.

```bash
# Run once to create migration tracking tables.
go run -tags saas cmd/csghub-server/main.go --config config.toml migration init

# Apply pending migrations.
go run -tags saas cmd/csghub-server/main.go --config config.toml migration migrate
```

The initial migrations create the OpenFGA tables, initialize the Store, write the models, and backfill relationships for existing data. Store and model 1.0 initialization is in `20260813140000_init_openfga_store_model.go`. The knowledge base model and relationship backfill are in `20260831171517_add_knowledge_base_rebac_model.go`.

Updating a model does not automatically grant access to resources. If the new model adds relations, backfill the relationships needed by existing resources in the migration as well.

## 4. Initializing ReBAC

Once the service has initialized the PostgreSQL database and completed the migrations, create an authorizer through the factory:

```go
// Import "opencsg.com/csghub-server/builder/rebac/factory".
authorizer, err := factory.NewAuthorizer()
if err != nil {
	return err
}
```

This uses the default authorization model, `OpenFgaAuthorizationModelIDLatest`. Pass the resulting `authorizer` to components that need permission checks; application code can use the `rebac.Authorizer` interface. The default Provider is reused, so you do not need to initialize it for each request or close it at the end of a request.

If you need to use a specific model, such as during a migration backfill, create a Provider yourself:

```go
// Import builder/rebac, builder/rebac/openfga, and common/types.
provider, err := openfga.NewCustomProvider(
	openfga.WithAuthorizationModelID(types.OpenFgaAuthorizationModelIDVer1_2),
)
if err != nil {
	return err
}
defer provider.Close()

authorizer, err := rebac.NewAuthorizer(provider)
if err != nil {
	return err
}
// Use authorizer before this task returns and closes the provider.
```

This example is suitable for a task function, with the Provider closed when the task ends. It uses the application database by default. If you have a separate pgx pool, also pass `openfga.WithPGXPool(pool)`.

## 5. Calling the Methods

The examples below grant a user read access to a repository. Put the snippets in an application function that returns `error`. Use the request context for `ctx` and the previously initialized instance for `authorizer`.

### identifiers.go: Preparing Users and Resources

[`identifiers.go`](identifiers.go) converts business UUIDs and IDs into ReBAC references. `Subject` represents who receives permission, and `Object` represents the resource the permission applies to. These methods only create references; they do not create business resources or grant access automatically.

First, prepare the user and repository references. Import `opencsg.com/csghub-server/builder/rebac`:

```go
// Use the user's UUID and the repository's database ID.
user := rebac.UserSubject(userUUID)
repository := rebac.RepositoryObject(42)
```

Use a UUID for the user and a database ID for the repository. Do not pass a username or repository path.

Use the other methods in this file to create references for other resources:

- `UserObject(userUUID)`: Treats a user account as a protected resource, for example when checking who can modify the account. Use `UserSubject` when granting access to a user, and `UserObject` when protecting a user account.
- `OrganizationObject(orgUUID)`: An organization resource, using the organization's UUID.
- `NamespaceObject(namespaceUUID)`: A namespace resource, using the namespace's UUID.
- `KnowledgeBaseObject(id)`: A knowledge base resource, using its database ID. Both this ID and the ID passed to `RepositoryObject(id)` must be greater than 0.
- `OrganizationMembers(orgUUID)`: The organization's member set, represented as `organization:<UUID>#member`. Use it as the Subject of a repository reader, writer, or admin relationship to grant access to all members of the organization.
- `OrganizationMembersFromChild(orgUUID)`: A member set that includes child organization members, represented as `organization:<UUID>#member_from_child`. Whether it can be written into a resource's relationship depends on the subject types allowed by the model. Repository roles currently allow `organization#member`.
- `AnonymousSubject()`: An anonymous user reference, represented as `user:anonymous`, for checking public read access by users who are not logged in.
- `PublicUserWildcard()`: A reference to all users, represented as `user:*`. For example, write it into a knowledge base's `public` relationship to make the knowledge base publicly readable.

If there is no dedicated constructor, use one of these general methods:

- `NewSubject(objectType, id)`: Creates a subject of the specified type. For example, use `NewSubject(ObjectTypeOrganization, orgUUID)` when an organization is the subject of a linking relationship.
- `NewObject(objectType, id)`: Creates a resource reference of the specified type.
- `NewUserset(objectType, id, relation)`: Creates a userset reference such as `type:id#relation`.
- `NewWildcardSubject(objectType)`: Creates a wildcard subject reference such as `type:*`. `PublicUserWildcard()` is its user type variant.

The file also provides `SystemObject()`, which returns `platform:platform`. The current default model and schema do not register platform, so add the corresponding definitions before using it.

When adding a resource type, you can add a dedicated constructor here so application code does not have to assemble the type and ID each time. Also register the type in the model and the ReBAC schema.

### Write: Granting Access

```go
// Grant the user the repository's reader relation.
relationships := []rebac.Relationship{{
	Subject:  user,
	Relation: rebac.RelationReader,
	Object:   repository,
}}
if err := authorizer.Write(ctx, relationships); err != nil {
	return err
}
```

This relationship means "the user is a reader of repository 42." You can also write `RelationWriter` or `RelationOwner`. Writes of existing relationships are ignored. A request can write up to 100 relationships; split larger sets into batches yourself. The slice must not be empty.

### Check: Checking Permission

```go
// Check effective read permission on the repository.
request := rebac.CheckRequest{
	Subject:     user,
	Relation:    rebac.RepositoryCanRead,
	Object:      repository,
	Consistency: rebac.ConsistencyHigher,
}
decision, err := authorizer.Check(ctx, request)
if err != nil {
	return err
}
if !decision.Allowed {
	return rebac.ErrDenied
}
```

Use `decision.Allowed` to see whether access is permitted. When access is denied, `err` is still `nil`, so checking only the error is not enough. This example checks `RepositoryCanRead`, which includes both direct grants and permissions inherited from organizations.

If you just need to block an operation when access is denied, use `Authorize` directly:

```go
// Return ErrDenied when the permission check is denied.
if err := authorizer.Authorize(ctx, request); err != nil {
	return err
}
```

### Delete: Revoking Access

Delete the same relationships written earlier:

```go
// Remove the same reader relation written above.
if err := authorizer.Delete(ctx, relationships); err != nil {
	return err
}
```

The user, relation, and object must match those used when writing the relationship. Missing relationships are ignored. Split bulk deletions into batches as well. After removing reader access, a user may still be able to read the repository through owner, writer, or inherited permissions. Use `Check` again to confirm.

### ListObjects: Listing Accessible Resources

The method is named `ListObjects`. For example, to list repositories the user can read, also import `fmt`:

```go
// List repositories the user can read.
result, err := authorizer.ListObjects(ctx, rebac.ListObjectsRequest{
	Subject:     user,
	Relation:    rebac.RepositoryCanRead,
	ObjectType:  rebac.ObjectTypeRepository,
	Consistency: rebac.ConsistencyHigher,
})
if err != nil {
	return err
}
for _, object := range result.Objects {
	fmt.Println(object.ID)
}
```

`result.Objects` contains object references with `Type` and `ID` fields. After obtaining the IDs, query the application database for repository details.

The default limit is 1000 objects. You can adjust it in the configuration:

```toml
[rebac]
openfga_list_object_max_result = 1000
```

Restart the service after changing the configuration. This method does not handle business pagination, and results are also affected by the query time limit. Do not treat the number of returned objects as the total number of resources the user can access.

Two other useful methods:

- `BatchCheck`: Checks multiple permissions at once, with a default limit of 50 items. Give each item a different `CorrelationID`. Read the results from `result.Results` and check each item's `Err` and `Decision.Allowed`.
- `ListSubjects`: Finds who has permission on an object. Pass `Object`, `Relation`, and `SubjectType`, and read the results from `result.Subjects`.

For more query options, see [OpenFGA relationship queries](https://openfga.dev/docs/interacting/relationship-queries).

## 6. Adding a Model Version and Updating Latest

Adding a model version saves a new set of permission rules to the database. `Latest` tells application code which set of rules to use by default. Adding a model to the database does not automatically switch application code to it.

The full constant name is `OpenFgaAuthorizationModelIDLatest`, defined in `common/types/rebac.go`. It currently points to version 1.2:

```go
// Use model version 1.2 for the default provider.
const OpenFgaAuthorizationModelIDLatest = OpenFgaAuthorizationModelIDVer1_2
```

For example, to add version 1.3:

1. Copy `dsl_v1_2.go` to `dsl_v1_3.go`, define `AuthorizationModelVer1_3`, and change the permission rules inside it.
2. Add `OpenFgaAuthorizationModelIDVer1_3` in `common/types/rebac.go`, using a newly generated ULID.
3. Add a migration that passes both `OpenFgaAuthorizationModelIDVer1_3` and `AuthorizationModelVer1_3` to `insertAuthorizationModel`. Run the migration to save the new model and backfill any required relationships. If you add types or relations, also update the ReBAC schema.
4. Once the new model and relationships are ready, change `Latest` as shown below, then rebuild and deploy the service:

```go
// Switch the default provider to model version 1.3 after its migration is applied.
const OpenFgaAuthorizationModelIDLatest = OpenFgaAuthorizationModelIDVer1_3
```

Version 1.3 here is an example of a new version. Add its DSL and ID constants first.

**Use a fixed version in migrations and Latest for the default application entry point.** For example, a migration for version 1.3 should explicitly use `OpenFgaAuthorizationModelIDVer1_3`, rather than `OpenFgaAuthorizationModelIDLatest`. Otherwise, if Latest later changes to 1.4, rerunning that migration could pair the version 1.3 DSL with the version 1.4 ID.

The default Provider created by `factory.NewAuthorizer()` stores the ID pointed to by Latest when it initializes. Subsequent `Write`, `Delete`, `Check`, and `ListObjects` calls all use this model. Latest is a constant in the code, so changing it requires rebuilding and deploying the service. A running service will not update automatically.

If you only want to use a new model in a particular task, first select it with `WithAuthorizationModelID(types.OpenFgaAuthorizationModelIDVer1_3)` without changing Latest immediately. Once you confirm the new model works, switch the default version used by the application. Before switching, make sure the corresponding ID has been saved to the database; otherwise, calls will fail because the model cannot be found.
