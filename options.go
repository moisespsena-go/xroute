package xroute

type OptionsInterface interface {
	Get(key interface{}) (value interface{}, ok bool)
	Set(key interface{}, value interface{})
	Delete(key interface{}) (ok bool)
	Options() map[interface{}]interface{}
}

type Options map[interface{}]interface{}

func (options Options) Get(key interface{}) (value interface{}, ok bool) {
	value, ok = options[key]
	return
}
func (options Options) Set(key interface{}, value interface{}) {
	options[key] = value
}
func (options Options) Delete(key interface{}) {
	delete(options, key)
}
func (options Options) Options() map[interface{}]interface{} {
	return options
}

func NewOptions(data ...map[interface{}]interface{}) Options {
	if len(data) == 1 {
		return Options(data[0])
	}
	return make(Options)
}
