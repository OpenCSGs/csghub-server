# User Management

## Deleting a User

An admin user can delete a regular user account. However, the system will perform a check before deletion. If the user meets any of the following conditions, the account cannot be deleted:

- The user owns an organization.  
- The user has running or building applications.  
- The user has billing data.  

After a user is deleted, the following data associated with the user will also be deleted:

- Model repositories, dataset repositories, code repositories, application space repositories, and Prompt repositories.  
- Namespace.  
- Access Tokens.  
- Organizations.  
- Organization members.  
- SSH Keys.  
- Prompt Conversations.
