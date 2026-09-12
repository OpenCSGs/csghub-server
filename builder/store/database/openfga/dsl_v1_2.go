package openfga

// AuthorizationModelVer1_2 contains the OpenFGA authorization model version 1.2.
const AuthorizationModelVer1_2 = `
model
  schema 1.1

type user
  relations
    define owner: [user]

    define can_read: owner
    define can_write: owner

type organization
  relations
    define parent: [organization]
    define child: [organization]

    define admin: [user]
    define writer: [user]
    define reader: [user]

    define can_admin: admin or can_admin from parent
    define can_write: can_admin or writer or can_write from parent
    define can_read: can_write or reader or can_read from parent

    define member: admin or writer or reader
    define member_from_child: member or member_from_child from child

type namespace
  relations
    define organization: [organization]

    define owner: [user]
    define admin: [user]
    define writer: [user]
    define reader: [user]

    define can_admin: owner or admin or can_admin from organization
    define can_write: can_admin or writer or can_write from organization
    define can_read: can_write or reader or can_read from organization

type repository
  relations
    define organization: [organization]
    define organization_direct: [organization]

    define owner: [user]
    define admin: [user, organization#member]
    define writer: [user, organization#member]
    define reader: [user, organization#member]

    define can_admin: owner or admin or can_admin from organization or admin from organization_direct
    define can_write: can_admin or writer or can_write from organization or writer from organization_direct
    define can_read: can_write or reader or can_read from organization or reader from organization_direct

type knowledge_base
  relations
    define namespace: [namespace]
    define public: [user:*]

    define can_admin: can_admin from namespace
    define can_write: can_write from namespace
    define can_read: public or can_read from namespace
`
