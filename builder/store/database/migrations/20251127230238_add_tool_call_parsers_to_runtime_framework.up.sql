-- Add tool_call_parsers column to runtime_frameworks table
ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS tool_call_parsers TEXT;

