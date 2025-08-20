SET statement_timeout = 0;

--bun:split

DELETE FROM prompt_prefixes WHERE
(kind = 'mcp_scan_summary') OR
(kind = 'tool_poison');