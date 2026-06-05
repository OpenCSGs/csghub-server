package errorx

type context map[string]any

func Ctx() context {
	ext := make(map[string]any)
	return ext
}

func (ctx context) Set(key string, value any) context {
	ctx[key] = value
	return ctx
}
