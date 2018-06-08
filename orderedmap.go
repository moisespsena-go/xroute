package route

type OrderedMapKey struct {
	Index int
	Key *string
	Values []*OrderedMapValue
}

type OrderedMapValue struct {
	Key *string
	Index int
	Value *string
}

type OrderedMap struct {
	Keys   []*string
	Values []*OrderedMapValue
	Map    map[string]*OrderedMapKey
	Size   int
}

func (p *OrderedMap) GetValue(key string) *OrderedMapValue {
	values, ok := p.Map[key]
	if ok {
		return values.Values[len(values.Values)-1]
	}
	return nil
}

func (p *OrderedMap) Get(key string) (value string) {
	if v := p.GetValue(key); v != nil {
		return *v.Value
	}
	return ""
}

func (p *OrderedMap) AddValue(value string) *OrderedMapValue {
	v := &OrderedMapValue{nil, len(p.Values), &value}
	p.Values = append(p.Values, v)
	return v
}

func (p *OrderedMap) AddKey(key string) *OrderedMapKey {
	k, ok := p.Map[key]
	if !ok {
		k = &OrderedMapKey{Index:len(p.Keys), Key:&key}
		p.Map[key] = k
		p.Keys = append(p.Keys, k.Key)
		p.Size = len(p.Keys)
	}
	return k
}

func (p *OrderedMap) Add(key, value string) {
	k := p.AddKey(key)
	v := p.AddValue(value)
	v.Key = k.Key
	k.Values = append(k.Values, v)
}

func (p *OrderedMap) Dict() map[string][]string {
	m := make(map[string][]string)
	for k, items := range p.Map {
		var data []string
		for _, v := range items.Values {
			data = append(data, *v.Value)
		}
		m[k] = data
	}
	return m
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{Map: make(map[string]*OrderedMapKey)}
}