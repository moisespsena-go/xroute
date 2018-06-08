package route

type TracedError interface {
	error
	Trace()[]byte
}
