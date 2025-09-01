package errorx

type context map[string]interface{}

func Ctx() context {
	ext := make(map[string]interface{})
	return ext
}

func (ctx context) Set(key string, value interface{}) context {
	ctx[key] = value
	return ctx
}
