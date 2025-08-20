SET statement_timeout = 0;

--bun:split

INSERT INTO tags(name, category, "group", scope, built_in, show_name) VALUES ('image-text-to-text', 'task', 'multimodal', 'model', true, '图文生成文本') ON CONFLICT (name, category, scope) DO UPDATE SET built_in = EXCLUDED.built_in;

INSERT INTO tags(name, category, "group", scope, built_in, show_name) VALUES ('text-to-video', 'task', 'computer_vision', 'model', true, '文本生成视频') ON CONFLICT (name, category, scope) DO UPDATE SET built_in = EXCLUDED.built_in;

INSERT INTO tags(name, category, "group", scope, built_in, show_name) VALUES ('video-text-to-text', 'task', 'multimodal', 'model', true, '视频文本生成文本') ON CONFLICT (name, category, scope) DO UPDATE SET built_in = EXCLUDED.built_in;

INSERT INTO tags(name, category, "group", scope, built_in, show_name) VALUES ('any-to-any', 'task', 'multimodal', 'model', true, '统一模态') ON CONFLICT (name, category, scope) DO UPDATE SET built_in = EXCLUDED.built_in;

INSERT INTO tags(name, category, "group", scope, built_in, show_name) VALUES ('audio-text-to-text', 'task', 'multimodal', 'model', true, '音频文本生成文本') ON CONFLICT (name, category, scope) DO UPDATE SET built_in = EXCLUDED.built_in;
