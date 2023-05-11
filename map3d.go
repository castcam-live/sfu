package main

type Map3D[K1 comparable, K2 comparable, K3 comparable, V any] map[K1]map[K2]map[K3]V

func (m Map3D[K1, K2, K3, V]) Get(key1 K1, key2 K2, key3 K3) (V, bool) {
	innerMap1, ok := m[key1]
	var d V
	if !ok {
		return d, false
	}

	innerMap2, ok := innerMap1[key2]
	if !ok {
		return d, false
	}

	value, ok := innerMap2[key3]
	if !ok {
		return d, false
	}

	return value, true
}

func (m Map3D[K1, K2, K3, V]) Set(key1 K1, key2 K2, key3 K3, value V) {
	innerMap1, ok := m[key1]
	if !ok {
		innerMap1 = map[K2]map[K3]V{}
		m[key1] = innerMap1
	}

	innerMap2, ok := innerMap1[key2]
	if !ok {
		innerMap2 = map[K3]V{}
		innerMap1[key2] = innerMap2
	}

	innerMap2[key3] = value
}

func (m Map3D[K1, K2, K3, V]) Remove(key1 K1, key2 K2, key3 K3) {
	innerMap1, ok := m[key1]
	if !ok {
		return
	}

	innerMap2, ok := innerMap1[key2]
	if !ok {
		return
	}

	delete(innerMap2, key3)

	if len(innerMap2) == 0 {
		delete(innerMap1, key2)
	}

	if len(innerMap1) == 0 {
		delete(m, key1)
	}
}
