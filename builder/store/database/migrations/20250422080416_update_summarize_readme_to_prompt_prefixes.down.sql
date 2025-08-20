SET statement_timeout = 0;
--bun:split
DELETE FROM prompt_prefixes WHERE
(kind = 'summarize_readme_model') OR
(kind = 'summarize_readme_dataset') OR
(kind = 'summarize_readme_code') OR
(kind = 'summarize_readme_space') OR
(kind = 'summarize_readme_prompt') OR
(kind = 'summarize_readme_mcpserver');