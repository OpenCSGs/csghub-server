package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		tags := []Tag{
			{
				Name:     "audio-text-to-video",
				Category: "task",
				Group:    "multimodal",
				Scope:    "model",
				ShowName: "音频文本生成视频",
				BuiltIn:  true,
			},
			{
				Name:     "audio-image-text-to-video",
				Category: "task",
				Group:    "multimodal",
				Scope:    "model",
				ShowName: "音频图像文本生成视频",
				BuiltIn:  true,
			},
			{
				Name:     "audio-driven-video-continuation",
				Category: "task",
				Group:    "multimodal",
				Scope:    "model",
				ShowName: "音频驱动视频续写",
				BuiltIn:  true,
			},
		}
		for _, tag := range tags {
			exists, err := db.NewSelect().Model((*Tag)(nil)).
				Where("name = ? AND category = ? AND scope = ?", tag.Name, tag.Category, tag.Scope).
				Exists(ctx)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			if _, err = db.NewInsert().Model(&tag).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewDelete().Model((*Tag)(nil)).
			Where("name IN (?)", bun.In([]string{
				"audio-text-to-video",
				"audio-image-text-to-video",
				"audio-driven-video-continuation",
			})).
			Where("category = ? AND scope = ? AND built_in = true", "task", "model").
			Exec(ctx)
		return err
	})
}
