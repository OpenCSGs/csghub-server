# Integration Test

This directory contains integration tests for the CSGHb server.

### Starting and Stopping the Test Environment

Setting up the test environment for integration testing is simple. Follow the steps below:

```go
	ctx := context.TODO()
	env, err := testinfra.StartTestEnv()
	defer func() { _ = env.Shutdown(ctx) }()
```

However, before proceeding with writing your tests, please read the following explanation to fully understand what happens in these three lines of code. A lot is happening behind the scenes.

When `testinfra.StartTestEnv()` is called, the following actions are performed in sequence:

1. **Load the configuration file**: The `common/config/test.toml` configuration file is loaded. This config is used during integration tests.
2. **Create a test PostgreSQL database**: A PostgreSQL database is created on a random port using test containers. The database configuration in the test config is updated accordingly.
3. **Start the Gitaly server**: A Gitaly server is started using test containers. The configuration used for Gitaly is either `tests/gitaly.toml` or `tests/gitaly_github.toml` (used when running on GitHub). Please add a comment in the code explaining why two config files are required. The Gitaly server configuration is updated once the container is started.
4. **Start the Temporal test server**: A local Temporal test server is started using the [temporaltest package](https://github.com/temporalio/temporal/blob/main/temporaltest/README.md). The workflow endpoint config is also updated. By default, the Temporal test server uses a random namespace to avoid conflicts, but we force the registration of the default namespace to ensure tests run.
5. **Start the in-memory S3 server**: A local in-memory S3 server is started using [GoFakeS3](https://github.com/johannesboyne/gofakes3). This server is used with the MinIO Go SDK for testing LFS (Large File Storage) functionality. The S3 configuration is updated accordingly.
6. **Start the Redis server**: A Redis server is started using test containers, and the Redis endpoint configuration is updated.
7. **Start the CSGHub server**: The CSGHub user server and its workflows are started.
8. **Start the CSGHub dataset viewer server**: The CSGHub dataset viewer server and its workflows are started.
9. **Start the CSGHub main API server**: The CSGHub main API server and its workflows are started.

That’s all. Note that not all services are started by default. For example, the NATS server or runner server is not started. If you need to test functionality related to these services, be sure to add them to the environment startup function.

After the test environment is started, always defer the call to `env.Shutdown(ctx)` to ensure all resources are properly cleaned up.

### Writing Tests

There are two test files provided:

- **model_test.go**: This tests CRUD operations for models and Git-related functionality. Since the model, dataset, space, and code all share the same repository and Git API code, you can consider this file to also test dataset, space, and code-related features.
- **dataset_viewer_test.go**: This file tests the Temporal workflows for the dataset viewer server.

The test code in these files clearly demonstrates how to test the API, Git, and workflows. There’s no need to repeat this here.

One important thing to remember is that for all integration tests, you should add the following snippet at the beginning of your test function:

```go
	if testing.Short() {
		t.Skip("skipping integration test")
	}
```

This allows you to differentiate between unit tests and integration tests.

### What to Test in Integration Tests

Integration tests involve starting multiple services and interacting with real databases (as opposed to database unit tests, which use in-memory or transactional databases and roll back changes after the test). Writing too many integration tests can significantly slow down the testing process, so here are three key suggestions:

1. **Group related actions into single test cases**: For example, basic CRUD operations can be tested in a single test case. However, avoid overloading tests. Separate unrelated actions, such as API and Git operations, into different tests.
2. **Prioritize the most used features**: Focus on testing the 80% of use cases that are most commonly used in your service. These are the most critical and should not fail.
3. **Test important but less common features**: For features that may not be used frequently but are critical (e.g., those that could result in significant issues, like financial losses), consider adding integration tests for them as well.

### Starting a Test API Server

You can also use the test environment to start a temporary test server. To do so, run the following command:

```
go run main.go start-test-env
```
