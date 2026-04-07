package errorx

const errMCPGatewayPrefix = "MCPGW-ERR"

const (
	gatewayMCPServerNameAlreadyExists = iota
	gatewayMCPServerInvalidName
)

var (
	// MCP server with the same name already exists in the gateway
	//
	// Description: An MCP server with this name already exists in the gateway.
	//
	// Description_ZH: 网关中已存在相同名称的MCP服务器。
	//
	// en-US: MCP server with the same name already exists: {{.server_name}}
	//
	// zh-CN: 网关中已存在相同名称的MCP服务器: {{.server_name}}
	//
	// zh-HK: 網關中已存在相同名稱的MCP服務器: {{.server_name}}
	ErrGatewayMCPServerNameAlreadyExists error = CustomError{prefix: errMCPGatewayPrefix, code: gatewayMCPServerNameAlreadyExists}
	// MCP server name is invalid
	//
	// Description: The MCP server name does not meet naming requirements.
	//
	// Description_ZH: MCP服务器名称不符合命名规范。
	//
	// en-US: invalid MCP server name: must start with a letter or digit, contain only letters, digits, underscores, or hyphens, and be at most 32 characters
	//
	// zh-CN: MCP服务器名称无效：必须以字母或数字开头，只能包含字母、数字、下划线或连字符，且最多32个字符
	//
	// zh-HK: MCP服務器名稱無效：必須以字母或數字開頭，只能包含字母、數字、下劃線或連字符，且最多32個字符
	ErrGatewayMCPServerInvalidName error = CustomError{prefix: errMCPGatewayPrefix, code: gatewayMCPServerInvalidName}
)

func GatewayMCPServerNameAlreadyExists(err error, ctx context) error {
	return CustomError{
		prefix:  errMCPGatewayPrefix,
		context: ctx,
		err:     err,
		code:    int(gatewayMCPServerNameAlreadyExists),
	}
}
