package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		tags := []database.Tag{
			{Name: "internet", Category: "industry", Group: "industry", Scope: "organization", ShowName: "互联网", I18nKey: "org_industry_internet", BuiltIn: true},
			{Name: "finance", Category: "industry", Group: "industry", Scope: "organization", ShowName: "金融", I18nKey: "org_industry_finance", BuiltIn: true},
			{Name: "healthcare", Category: "industry", Group: "industry", Scope: "organization", ShowName: "医疗健康", I18nKey: "org_industry_healthcare", BuiltIn: true},
			{Name: "education", Category: "industry", Group: "industry", Scope: "organization", ShowName: "教育", I18nKey: "org_industry_education", BuiltIn: true},
			{Name: "manufacturing", Category: "industry", Group: "industry", Scope: "organization", ShowName: "制造业", I18nKey: "org_industry_manufacturing", BuiltIn: true},
			{Name: "transportation", Category: "industry", Group: "industry", Scope: "organization", ShowName: "交通运输", I18nKey: "org_industry_transportation", BuiltIn: true},
			{Name: "energy", Category: "industry", Group: "industry", Scope: "organization", ShowName: "能源", I18nKey: "org_industry_energy", BuiltIn: true},
			{Name: "agriculture", Category: "industry", Group: "industry", Scope: "organization", ShowName: "农业", I18nKey: "org_industry_agriculture", BuiltIn: true},
			{Name: "retail", Category: "industry", Group: "industry", Scope: "organization", ShowName: "零售", I18nKey: "org_industry_retail", BuiltIn: true},
			{Name: "media", Category: "industry", Group: "industry", Scope: "organization", ShowName: "媒体", I18nKey: "org_industry_media", BuiltIn: true},
			{Name: "telecom", Category: "industry", Group: "industry", Scope: "organization", ShowName: "电信", I18nKey: "org_industry_telecom", BuiltIn: true},
			{Name: "legal", Category: "industry", Group: "industry", Scope: "organization", ShowName: "法律", I18nKey: "org_industry_legal", BuiltIn: true},
			{Name: "government", Category: "industry", Group: "industry", Scope: "organization", ShowName: "政府", I18nKey: "org_industry_government", BuiltIn: true},
			{Name: "real_estate", Category: "industry", Group: "industry", Scope: "organization", ShowName: "房地产", I18nKey: "org_industry_real_estate", BuiltIn: true},
			{Name: "biotech", Category: "industry", Group: "industry", Scope: "organization", ShowName: "生物技术", I18nKey: "org_industry_biotech", BuiltIn: true},
			{Name: "semiconductor", Category: "industry", Group: "industry", Scope: "organization", ShowName: "半导体", I18nKey: "org_industry_semiconductor", BuiltIn: true},
			{Name: "automotive", Category: "industry", Group: "industry", Scope: "organization", ShowName: "汽车", I18nKey: "org_industry_automotive", BuiltIn: true},
			{Name: "game", Category: "industry", Group: "industry", Scope: "organization", ShowName: "游戏", I18nKey: "org_industry_game", BuiltIn: true},
			{Name: "cybersecurity", Category: "industry", Group: "industry", Scope: "organization", ShowName: "网络安全", I18nKey: "org_industry_cybersecurity", BuiltIn: true},
			{Name: "other", Category: "industry", Group: "industry", Scope: "organization", ShowName: "其他", I18nKey: "org_industry_other", BuiltIn: true},
		}

		for _, tag := range tags {
			exists, err := db.NewSelect().Model((*database.Tag)(nil)).
				Where("name = ? AND category = ? AND scope = ?", tag.Name, tag.Category, tag.Scope).
				Exists(ctx)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			_, err = db.NewInsert().Model(&tag).Exec(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return nil
	})
}
