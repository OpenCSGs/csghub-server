CSGHub server supports two configuration methods: using environment variables or configuration files. You can also combine them, for example, by placing non-sensitive configurations in a config file and overriding sensitive configurations with environment variables.

### Using a Config File

CSGHub supports TOML format for config files. When starting any service from the command line, you can specify the config file with the `--config` option:

```
go run cmd/csghub-server/main.go start server --config local.toml
go run cmd/csghub-server/main.go deploy runner --config local.toml
go run cmd/csghub-server/main.go mirror repo-sync --config common/config/test.toml
```

We provide an [example config file](common/config/config.toml.example), you can rename it, modify as needed and use. All available configurations are defined in [this Go file](common/config/config.go). The TOML configuration uses snake_case naming convention, and names automatically map to corresponding struct field names.

If a config value is missing in the config file, the `default` tag value specified in the Go struct file will be used.

### Using Environment Variables

CSGHub also supports configuration through environment variables. The relevant environment variable names are defined in [this Go file](common/config/config.go) under the `env` tag. If an environment variable is absent, the value in the `default` tag will be used.

### Combining Config File with Environment Variables

You can use config file together with environment variables. When both are used, environment variables take **higher priority** than the config file. For example, if you have a `Port` setting, and you specify it in the TOML file as `port=1234` and in an environment variable as `export PORT=5678`, the environment variable value (5678) will be used for the port configuration.
