package my

func RandomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(RandIntBetween(65, 90))
	}
	return string(bytes)
}
