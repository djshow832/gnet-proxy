package util

func Try(xs ...interface{}) interface{} {
	if len(xs) == 0 {
		return nil
	}
	if err, ok := xs[len(xs)-1].(error); ok && err != nil {
		panic(err)
	}
	return xs[0]
}
