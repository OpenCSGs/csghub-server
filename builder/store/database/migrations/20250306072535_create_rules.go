package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types/enum"
)

type Rule struct {
	ID       int64         `bun:"id,pk,autoincrement"`
	Content  string        `bun:",notnull"`
	RuleType enum.RuleType `bun:",notnull,unique"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &Rule{})
		if err != nil {
			return err
		}

		return initGenSyncVersionRule(db)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &Rule{})
	})
}

func initGenSyncVersionRule(db *bun.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rule := &Rule{
		Content:  genSyncVersionContent,
		RuleType: enum.GenSyncVersion,
	}
	_, err := db.NewInsert().Model(rule).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert gen sync version rule")
	}
	return nil
}

const genSyncVersionContent = `
	result := false
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
`
