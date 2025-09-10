package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type mirrorNamespaceMapping struct {
	ID              int    `bun:",pk,autoincrement"`
	SourceNamespace string `bun:",notnull,unique"`
	TargetNamespace string `bun:",notnull"`
	Enabled         bool   `bun:",notnull,default:true"`

	times
}

var mirrorOrganizationMap = map[string]string{
	"THUDM":           "THUDM",
	"baichuan-inc":    "BaiChuanAI",
	"IDEA-CCNL":       "FengShenBang",
	"internlm":        "ShangHaiAILab",
	"pleisto":         "Pleisto",
	"01-ai":           "01AI",
	"codefuse-ai":     "codefuse-ai",
	"WisdomShell":     "WisdomShell",
	"microsoft":       "microsoft",
	"Skywork":         "Skywork",
	"BAAI":            "BAAI",
	"deepseek-ai":     "deepseek-ai",
	"WizardLMTeam":    "WizardLM",
	"IEITYuan":        "IEITYuan",
	"Qwen":            "Qwen",
	"TencentARC":      "TencentARC",
	"OrionStarAI":     "OrionStarAI",
	"openbmb":         "OpenBMB",
	"netease-youdao":  "Netease-youdao",
	"ByteDance":       "ByteDance",
	"opencompass":     "opencompass",
	"Wan-AI":          "Wan-AI",
	"ByteDance-Seed":  "ByteDance-Seed",
	"xai-org":         "xai-org",
	"OpenGVLab":       "OpenGVLab",
	"ZhipuAI":         "ZhipuAI",
	"Tencent-Hunyuan": "Tencent-Hunyuan",
	"XiaomiMiMo":      "XiaomiMiMo",
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &mirrorNamespaceMapping{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model(&mirrorNamespaceMapping{}).
			Index("idx_mirror_namespace_mapping_soure_namespace_enabled").
			Column("source_namespace", "enabled").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for mirror_namespace_mapping on source_namespace/enabled")
		}
		var mirrorNamespaceMappings []mirrorNamespaceMapping
		for source, target := range mirrorOrganizationMap {
			mirrorNamespaceMappings = append(mirrorNamespaceMappings, mirrorNamespaceMapping{
				SourceNamespace: source,
				TargetNamespace: target,
				Enabled:         true,
			})
		}
		return db.NewInsert().Model(&mirrorNamespaceMappings).Scan(ctx)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &mirrorNamespaceMapping{})
	})
}
