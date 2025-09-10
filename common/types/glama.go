package types

type MCPResp struct {
	Servers  []GlamaMCPServer `json:"servers"` // Assuming the response contains a list of MCPs under the key "servers"
	PageInfo PageInfo         `json:"pageInfo"`
}

type PageInfo struct {
	EndCursor       string `json:"endCursor"`
	HasNextPage     bool   `json:"hasNextPage"`     // Indicates if there are more pages to fetch
	HasPreviousPage bool   `json:"hasPreviousPage"` // Indicates if there are previous pages to fetch
	StartCursor     string `json:"startCursor"`     // Cursor for the first item in the current page
}

type GlamaMCPServer struct {
	Attributes  []string      `json:"attributes"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Repository  MCPRepository `json:"repository"`
	License     License       `json:"spdxLicense"`                    // SPDX License Identifier
	Schema      MCPSchema     `json:"environmentVariablesJsonSchema"` // JSON Schema for the MCP's data structure
	Tools       []MCPTool     `json:"tools"`
}

type MCPRepository struct {
	URL string `json:"url"`
}

type License struct {
	Name string `json:"name"` // SPDX License Identifier, e.g., "MIT" or "Apache-2.0"
	URL  string `json:"url"`  // URL to the license text, if available. Optional.
}

type MCPSchema struct {
	Properties map[string]MCPSchemaProperty `json:"properties"`
	Required   []string                     `json:"required"` // Optional, list of required properties
	Type       string                       `json:"type"`     // Should be "object" for a schema representing an object with properties. Optional, but recommended for clarity.
}

type MCPSchemaProperty struct {
	Type        any    `json:"type"`
	Description string `json:"description"`
	Required    any    `json:"required"` // Optional, indicates if the property is required
}

type MCPTool struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	InputSchema MCPToolSchema `json:"inputSchema"`
}

type MCPToolSchema struct {
	Schema               string   `json:"$schema"`              // JSON Schema for the tool's input data structure
	AdditionalProperties bool     `json:"additionalProperties"` // Optional, indicates if additional properties are allowed
	Properties           any      `json:"properties"`
	Required             []string `json:"required"` // Optional, list of required properties
	Type                 any      `json:"type"`     // Should be "object" for a schema representing an object with properties. Optional, but recommended for clarity.
}
