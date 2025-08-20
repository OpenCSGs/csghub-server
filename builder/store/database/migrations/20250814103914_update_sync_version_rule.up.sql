SET statement_timeout = 0;

--bun:split

UPDATE rules SET content = $$result := false
	namespaces := [
	    "AIWizards", 
		"THUDM",
		"BaiChuanAI",
		"FengShenBang",
		"ShangHaiAILab",
		"Pleisto",
		"01AI",
		"codefuse-ai",
		"WisdomShell",
		"microsoft",
		"Skywork",
		"BAAI",
		"WizardLM",
		"IEITYuan",
		"Qwen",
		"deepseek",
		"TencentARC",
		"ShengtengModelZoo",
		"OrionStarAI",
		"OpenBMB",
		"Netease-youdao",
		"iFlytek",
		"FreedomAI",
		"ByteDance",
		"EPFL-VILAB",
		"Open-Sora",
		"OpenGithubs",
		"OpenGithub",
		"deepseek-ai",
		"black-forest-labs",
		"LGAI-EXAONE",
		"nvidia",
		"hexgrad",
		"mistral-community",
		"stepfun-ai",
		"meta-llama",
		"InternLM",
		"rainbow1011",
		"rain1011",
		"apple",
		"opencompass",
		"genmo",
		"stabilityai",
		"CohereForAI",
		"facebook",
		"rhymes-ai",
		"infly",
		"briaai",
		"Lightricks",
		"AIDC-AI",
		"tencent",
		"simplescaling",
		"agentica-org",
		"OpenCSG",
		"DeepseekAI",
		"deepseek-ai",
		"billionaire",
		"MagicAI"
	]
	contains := func(s, e) {
		for a in s {
			if a == e {
				return true
			}
		}
		return false
	}
	if status == "finished" {
		result = true
	} else if status == "" {
	    if contains(namespaces, namespace) {
		  result = true
		}
	}
	if repo_type == "space" {
		result = false
	}$$ WHERE rule_type = 'gen_sync_version';

