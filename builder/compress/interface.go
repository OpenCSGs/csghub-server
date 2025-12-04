package compress

type encode func([]byte) ([]byte, error)
type decode func([]byte) ([]byte, error)

func Encode(encodeType string, data []byte) ([]byte, error) {
	var encodeFunc encode
	switch encodeType {
	case "gzip":
		encodeFunc = gzipEncode
	case "deflate":
		encodeFunc = deflateEncode
	case "br":
		encodeFunc = brEncode
	default:
		encodeFunc = defaultEncode
	}
	return encodeFunc(data)
}

func Decode(encodeType string, data []byte) ([]byte, error) {
	var decodeFunc decode
	switch encodeType {
	case "gzip":
		decodeFunc = gzipDecode
	case "deflate":
		decodeFunc = deflateDecode
	case "br":
		decodeFunc = brDecode
	default:
		decodeFunc = defaultDecode
	}
	return decodeFunc(data)
}

func defaultEncode(data []byte) ([]byte, error) {
	return data, nil
}

func defaultDecode(data []byte) ([]byte, error) {
	return data, nil
}
