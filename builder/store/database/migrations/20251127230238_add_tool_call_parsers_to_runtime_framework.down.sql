-- Remove tool_call_parsers column from runtime_frameworks table
ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS tool_call_parsers;

