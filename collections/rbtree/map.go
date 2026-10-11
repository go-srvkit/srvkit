// Copyright © 2023-2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package treemap

import (
	"cmp"
)

// Map is a Red-Black tree based map implementation.
// The map is sorted by a comparator passed to its constructor.
// This implementation provides guaranteed log(n) time cost for the
// ContainsKey(), Get(), Put() and Remove() operations.
//
// detail code taken from JDK, see links below
// https://github.com/openjdk/jdk/blob/jdk-11+28/src/java.base/share/classes/java/util/TreeMap.java

// Color Red-black mechanics
type Color uint8

const (
	RED   Color = 0
	BLACK Color = 1
)

func (c Color) String() string {
	switch c {
	case RED:
		return "red"
	case BLACK:
		return "black"
	default:
		return "??"
	}
}

type Entry[K, V comparable] struct {
	left, right, parent *Entry[K, V]
	color               Color
	key                 K
	value               V
}

func NewEntry[K, V comparable](key K, val V, parent *Entry[K, V]) *Entry[K, V] {
	return &Entry[K, V]{
		key:    key,
		value:  val,
		parent: parent,
		color:  BLACK,
	}
}

func (e *Entry[K, V]) GetKey() K {
	return e.key
}

func (e *Entry[K, V]) GetValue() V {
	return e.value
}

func (e *Entry[K, V]) SetValue(val V) V {
	var old = e.value
	e.value = val
	return old
}

func (e *Entry[K, V]) Height() int {
	if e == nil {
		return 0
	}
	var leftHeight = e.left.Height()
	var rightHeight = e.right.Height()
	if leftHeight > rightHeight {
		return leftHeight + 1
	} else {
		return rightHeight + 1
	}
}

type Comparator[T any] func(a, b T) int

// Map 红黑树map，引用语义，非线程安全；使用`New`创建，创建后不要复制
type Map[K, V comparable] struct {
	root       *Entry[K, V]
	comparator Comparator[K]
	size       int // The number of entries in the tree
	version    int // The number of structural modifications to the tree.
}

func New[K, V comparable](comparator Comparator[K]) *Map[K, V] {
	if comparator == nil {
		return nil
	}
	return &Map[K, V]{
		comparator: comparator,
	}
}

func NewOrderedTreeMap[K cmp.Ordered, V comparable]() *Map[K, V] {
	return &Map[K, V]{
		comparator: cmp.Compare[K],
	}
}

func NewTreeMapOf[M ~map[K]V, K cmp.Ordered, V comparable](unordered M) *Map[K, V] {
	var m = &Map[K, V]{
		comparator: cmp.Compare[K],
	}
	for k, v := range unordered {
		m.Put(k, v)
	}
	return m
}

// Size returns the number of key-value mappings in this map.
func (m *Map[K, V]) Size() int {
	return m.size
}

func (m *Map[K, V]) IsEmpty() bool {
	return m.size == 0
}

// ContainsKey return true if this map contains a mapping for the specified key
func (m *Map[K, V]) ContainsKey(key K) bool {
	return m.getEntry(key) != nil
}

// Get returns the value to which the specified key is mapped,
// or nil if this map contains no mapping for the key.
func (m *Map[K, V]) Get(key K) (V, bool) {
	var p = m.getEntry(key)
	if p != nil {
		return p.value, true
	}
	var zero V
	return zero, false
}

func (m *Map[K, V]) Load(key K) (V, bool) {
	return m.Get(key)
}

// GetOrDefault returns the value to which the specified key is mapped,
// or `defaultValue` if this map contains no mapping for the key.
func (m *Map[K, V]) GetOrDefault(key K, defVal V) V {
	var p = m.getEntry(key)
	if p != nil {
		return p.value
	}
	return defVal
}

// FirstKey returns the first key in the Map (according to the key's order)
func (m *Map[K, V]) FirstKey() (K, bool) {
	return keyOf[K](m.getFirstEntry())
}

// LastKey returns the last key in the Map (according to the key's order)
func (m *Map[K, V]) LastKey() (K, bool) {
	return keyOf(m.getLastEntry())
}

// RootEntry 根节点，外部保留引用会有删除后不一致的风险
func (m *Map[K, V]) RootEntry() *Entry[K, V] {
	return m.root
}

// FirstEntry 平衡树最左节点，外部保留引用会有删除后不一致的风险
func (m *Map[K, V]) FirstEntry() *Entry[K, V] {
	return m.getFirstEntry()
}

// LastEntry 平衡树最右节点，外部保留引用会有删除后不一致的风险
func (m *Map[K, V]) LastEntry() *Entry[K, V] {
	return m.getLastEntry()
}

// FloorEntry gets the entry corresponding to the specified key;
// if no such entry exists, returns the entry for the greatest key less than the specified key;
func (m *Map[K, V]) FloorEntry(key K) *Entry[K, V] {
	return m.getFloorEntry(key)
}

// FloorKey gets the specified key, returns the greatest key less than the specified key if not exist.
func (m *Map[K, V]) FloorKey(key K) (K, bool) {
	var entry = m.getFloorEntry(key)
	if entry != nil {
		return entry.key, true
	}
	var zero K
	return zero, false
}

// CeilingEntry gets the entry corresponding to the specified key;
// returns the entry for the least key greater than the specified key if not exist.
func (m *Map[K, V]) CeilingEntry(key K) *Entry[K, V] {
	return m.getCeilingEntry(key)
}

// CeilingKey gets the specified key, return the least key greater than the specified key if not exist.
func (m *Map[K, V]) CeilingKey(key K) (K, bool) {
	var entry = m.getCeilingEntry(key)
	if entry != nil {
		return entry.key, true
	}
	var zero K
	return zero, false
}

// HigherEntry gets the entry for the least key greater than the specified key
func (m *Map[K, V]) HigherEntry(key K) *Entry[K, V] {
	return m.getHigherEntry(key)
}

// HigherKey returns the least key greater than the specified key
func (m *Map[K, V]) HigherKey(key K) (K, bool) {
	var entry = m.getHigherEntry(key)
	if entry != nil {
		return entry.key, true
	}
	var zero K
	return zero, false
}

// Foreach performs the given action for each entry in this map until all entries
// have been processed or the action panic
func (m *Map[K, V]) Foreach(visit KeyValVisitor[K, V]) {
	var ver = m.version
	for e := m.getFirstEntry(); e != nil; e = successor(e) {
		goOn := visit(e.key, e.value)
		if ver != m.version {
			panic("concurrent map modification")
		}
		if !goOn {
			break
		}
	}
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (m *Map[K, V]) Range(visit func(key K, value V) (shouldContinue bool)) {
	var ver = m.version
	for e := m.getFirstEntry(); e != nil; e = successor(e) {
		goOn := visit(e.key, e.value)
		if ver != m.version {
			panic("concurrent map modification")
		}
		if !goOn {
			break
		}
	}
}

// Keys return list of all keys
func (m *Map[K, V]) Keys() []K {
	var keys = make([]K, 0, m.size)
	for e := m.getFirstEntry(); e != nil; e = successor(e) {
		keys = append(keys, e.key)
	}
	return keys
}

// Values return list of all values
func (m *Map[K, V]) Values() []V {
	var values = make([]V, 0, m.size)
	for e := m.getFirstEntry(); e != nil; e = successor(e) {
		values = append(values, e.value)
	}
	return values
}

// ToSysMap returns a map[K]V
func (m *Map[K, V]) ToSysMap() map[K]V {
	var unordered = make(map[K]V, m.size)
	for e := m.getFirstEntry(); e != nil; e = successor(e) {
		unordered[e.key] = e.value
	}
	return unordered
}

//func (m *Map[K, V]) Iterator() algext.Iterator[*Entry[K, V]] {
//	return NewEntryIterator(m, m.getFirstEntry())
//}
//
//func (m *Map[K, V]) DescendingIterator() algext.Iterator[*Entry[K, V]] {
//	return NewKeyDescendingEntryIterator(m, m.getLastEntry())
//}
//
//func (m *Map[K, V]) KeyIterator() algext.Iterator[K] {
//	return NewKeyIterator(m, m.getFirstEntry())
//}
//
//func (m *Map[K, V]) DescendingKeyIterator() algext.Iterator[K] {
//	return NewDescendingKeyIterator(m, m.getLastEntry())
//}
//
//func (m *Map[K, V]) ValueIterator() algext.Iterator[V] {
//	return NewValueIterator(m, m.getFirstEntry())
//}

// Put associates the specified value with the specified key in this map.
// If the map previously contained a mapping for the key, the old value is replaced.
func (m *Map[K, V]) Put(key K, value V) V {
	previous, _ := m.put(key, value, true)
	return previous
}

// Store sets the value for a key, equivalent to Put.
func (m *Map[K, V]) Store(key K, value V) {
	m.put(key, value, true)
}

// Swap swaps the value for a key and returns the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	return m.put(key, value, true)
}

// PutIfAbsent put a key-value pair if the key is not already associated with a value.
func (m *Map[K, V]) PutIfAbsent(key K, value V) V {
	previous, _ := m.put(key, value, false)
	return previous
}

// Remove removes the mapping for this key from this Map if present.
func (m *Map[K, V]) Remove(key K) bool {
	var p = m.getEntry(key)
	if p != nil {
		m.deleteEntry(p)
		return true
	}
	return false
}

func (m *Map[K, V]) Delete(key K) {
	m.Remove(key)
}

// Clear removes all the mappings from this map.
func (m *Map[K, V]) Clear() {
	m.version++
	m.size = 0
	m.root = nil
}

// Returns the first Entry in the Map (according to the key's order)
// Returns nil if the Map is empty.
func (m *Map[K, V]) getFirstEntry() *Entry[K, V] {
	var p = m.root
	if p != nil {
		for p.left != nil {
			p = p.left
		}
	}
	return p
}

// Returns the last Entry in the Map (according to the key's order)
// Returns nil if the Map is empty.
func (m *Map[K, V]) getLastEntry() *Entry[K, V] {
	var p = m.root
	if p != nil {
		for p.right != nil {
			p = p.right
		}
	}
	return p
}

// Returns this map's entry for the given key,
// or nil if the map does not contain an entry for the key.
func (m *Map[K, V]) getEntry(key K) *Entry[K, V] {
	var p = m.root
	for p != nil {
		var cmp = m.comparator(key, p.key)
		if cmp < 0 {
			p = p.left
		} else if cmp > 0 {
			p = p.right
		} else {
			return p
		}
	}
	return nil
}

// Gets the entry corresponding to the specified key;
// if no such entry exists, returns the entry for the least key greater than the specified key;
// if no such entry exists returns nil.
func (m *Map[K, V]) getCeilingEntry(key K) *Entry[K, V] {
	var p = m.root
	for p != nil {
		var cmp = m.comparator(key, p.key)
		if cmp < 0 {
			if p.left != nil {
				p = p.left
			} else {
				return p
			}
		} else if cmp > 0 {
			if p.right != nil {
				p = p.right
			} else {
				var parent = p.parent
				var ch = p
				for parent != nil && ch == parent.right {
					ch = parent
					parent = parent.parent
				}
				return parent
			}
		} else {
			return p
		}
	}
	return nil
}

// Gets the entry corresponding to the specified key;
// if no such entry exists, returns the entry for the greatest key less than the specified key;
// if no such entry exists, returns nil.
func (m *Map[K, V]) getFloorEntry(key K) *Entry[K, V] {
	var p = m.root
	for p != nil {
		var cmp = m.comparator(key, p.key)
		if cmp > 0 {
			if p.right != nil {
				p = p.right
			} else {
				return p
			}
		} else if cmp < 0 {
			if p.left != nil {
				p = p.left
			} else {
				var parent = p.parent
				var ch = p
				for parent != nil && ch == parent.left {
					ch = parent
					parent = parent.parent
				}
				return parent
			}
		} else {
			return p
		}

	}
	return nil
}

// Gets the entry for the least key greater than the specified key;
// if no such entry exists, returns the entry for the least key greater than the specified key;
// if no such entry exists returns nil.
func (m *Map[K, V]) getHigherEntry(key K) *Entry[K, V] {
	var p = m.root
	for p != nil {
		var cmp = m.comparator(key, p.key)
		if cmp < 0 {
			if p.left != nil {
				p = p.left
			} else {
				return p
			}
		} else {
			if p.right != nil {
				p = p.right
			} else {
				var parent = p.parent
				var ch = p
				for parent != nil && ch == parent.right {
					ch = parent
					parent = parent.parent
				}
				return parent
			}
		}
	}
	return nil
}

// Returns the entry for the greatest key less than the specified key;
// if no such entry exists (i.e., the least key in the Tree is greater than the specified key), returns nil
func (m *Map[K, V]) getLowerEntry(key K) *Entry[K, V] {
	var p = m.root
	for p != nil {
		var cmp = m.comparator(key, p.key)
		if cmp > 0 {
			if p.right != nil {
				p = p.right
			} else {
				return p
			}
		} else {
			if p.left != nil {
				p = p.left
			} else {
				var parent = p.parent
				var ch = p
				for parent != nil && ch == parent.left {
					ch = parent
					parent = parent.parent
				}
				return parent
			}
		}
	}
	return nil
}

func (m *Map[K, V]) put(key K, value V, replaceOld bool) (previous V, existed bool) {
	var t = m.root
	if t == nil {
		m.addEntryToEmptyMap(key, value)
		return previous, false
	}
	var cmp = 0
	var parent *Entry[K, V]
	for {
		parent = t
		cmp = m.comparator(key, t.key)
		if cmp < 0 {
			t = t.left
		} else if cmp > 0 {
			t = t.right
		} else {
			previous = t.value
			if replaceOld {
				t.value = value
			}
			return previous, true
		}
		if t == nil {
			break
		}
	}
	m.addEntry(key, value, parent, cmp < 0)
	return previous, false
}

func (m *Map[K, V]) addEntry(key K, value V, parent *Entry[K, V], addToLeft bool) {
	var entry = NewEntry(key, value, parent)
	if addToLeft {
		parent.left = entry
	} else {
		parent.right = entry
	}
	m.fixAfterInsertion(entry)
	m.size++
	m.version++
}

func (m *Map[K, V]) addEntryToEmptyMap(key K, value V) {
	m.root = NewEntry(key, value, nil)
	m.size = 1
	m.version++
}

func (m *Map[K, V]) deleteEntry(p *Entry[K, V]) {
	m.version++
	m.size--

	// If strictly internal, copy successor's element to p and then make p
	// point to successor.
	if p.left != nil && p.right != nil {
		var s = successor(p)
		p.key = s.key
		p.value = s.value
		p = s
	} // p has 2 children

	// Start fixup at replacement node, if it exists.
	var replacement = p.left
	if p.left == nil {
		replacement = p.right
	}

	if replacement != nil {
		// Link replacement to parent
		replacement.parent = p.parent
		if p.parent == nil {
			m.root = replacement
		} else if p == p.parent.left {
			p.parent.left = replacement
		} else {
			p.parent.right = replacement
		}

		// Null out links, so they are OK to use by fixAfterDeletion.
		p.left = nil
		p.right = nil
		p.parent = nil

		// Fix replacement
		if p.color == BLACK {
			m.fixAfterDeletion(replacement)
		}
	} else if p.parent == nil { // return if we are the only node.
		m.root = nil
	} else { //  No children. Use self as phantom replacement and unlink.
		if p.color == BLACK {
			m.fixAfterDeletion(p)
		}
		if p.parent != nil {
			if p == p.parent.left {
				p.parent.left = nil
			} else if p == p.parent.right {
				p.parent.right = nil
			}
			p.parent = nil
		}
	}
}

// Balancing operations.
//
// Implementations of rebalancings during insertion and deletion are
// slightly different from the Cormen, Leiserson, and Rivest's <Introduction to Algorithms> version.
// Rather than using dummy nil nodes, we use a set of accessors that deal properly with nil.
// They are used to avoid messiness surrounding nullness checks in the main algorithms.
//
// see original version at http://staff.ustc.edu.cn/~csli/graduate/algorithms/book6/chap14.htm

func (m *Map[K, V]) rotateLeft(p *Entry[K, V]) {
	if p != nil {
		var r = p.right
		p.right = r.left
		if r.left != nil {
			r.left.parent = p
		}
		r.parent = p.parent
		if p.parent == nil {
			m.root = r
		} else if p.parent.left == p {
			p.parent.left = r
		} else {
			p.parent.right = r
		}
		r.left = p
		p.parent = r
	}
}

func (m *Map[K, V]) rotateRight(p *Entry[K, V]) {
	if p != nil {
		var l = p.left
		p.left = l.right
		if l.right != nil {
			l.right.parent = p
		}
		l.parent = p.parent
		if p.parent == nil {
			m.root = l
		} else if p.parent.right == p {
			p.parent.right = l
		} else {
			p.parent.left = l
		}
		l.right = p
		p.parent = l
	}
}

func (m *Map[K, V]) fixAfterInsertion(x *Entry[K, V]) {
	x.color = RED
	for x != nil && x != m.root && x.parent.color == RED {
		if parentOf(x) == leftOf(parentOf(parentOf(x))) {
			var y = rightOf(parentOf(parentOf(x)))
			if colorOf(y) == RED {
				setColor(parentOf(x), BLACK)
				setColor(y, BLACK)
				setColor(parentOf(parentOf(x)), RED)
				x = parentOf(parentOf(x))
			} else {
				if x == rightOf(parentOf(x)) {
					x = parentOf(x)
					m.rotateLeft(x)
				}
				setColor(parentOf(x), BLACK)
				setColor(parentOf(parentOf(x)), RED)
				m.rotateRight(parentOf(parentOf(x)))
			}
		} else {
			var y = leftOf(parentOf(parentOf(x)))
			if colorOf(y) == RED {
				setColor(parentOf(x), BLACK)
				setColor(y, BLACK)
				setColor(parentOf(parentOf(x)), RED)
				x = parentOf(parentOf(x))
			} else {
				if x == leftOf(parentOf(x)) {
					x = parentOf(x)
					m.rotateRight(x)
				}
				setColor(parentOf(x), BLACK)
				setColor(parentOf(parentOf(x)), RED)
				m.rotateLeft(parentOf(parentOf(x)))
			}
		}
	}
	m.root.color = BLACK
}

func (m *Map[K, V]) fixAfterDeletion(x *Entry[K, V]) {
	for x != m.root && colorOf(x) == BLACK {
		if x == leftOf(parentOf(x)) {
			var sib = rightOf(parentOf(x))

			if colorOf(sib) == RED {
				setColor(sib, BLACK)
				setColor(parentOf(x), RED)
				m.rotateLeft(parentOf(x))
				sib = rightOf(parentOf(x))
			}

			if colorOf(leftOf(sib)) == BLACK &&
				colorOf(rightOf(sib)) == BLACK {
				setColor(sib, RED)
				x = parentOf(x)
			} else {
				if colorOf(rightOf(sib)) == BLACK {
					setColor(leftOf(sib), BLACK)
					setColor(sib, RED)
					m.rotateRight(sib)
					sib = rightOf(parentOf(x))
				}
				setColor(sib, colorOf(parentOf(x)))
				setColor(parentOf(x), BLACK)
				setColor(rightOf(sib), BLACK)
				m.rotateLeft(parentOf(x))
				x = m.root
			}
		} else { // symmetric
			var sib = leftOf(parentOf(x))

			if colorOf(sib) == RED {
				setColor(sib, BLACK)
				setColor(parentOf(x), RED)
				m.rotateRight(parentOf(x))
				sib = leftOf(parentOf(x))
			}

			if colorOf(rightOf(sib)) == BLACK &&
				colorOf(leftOf(sib)) == BLACK {
				setColor(sib, RED)
				x = parentOf(x)
			} else {
				if colorOf(leftOf(sib)) == BLACK {
					setColor(rightOf(sib), BLACK)
					setColor(sib, RED)
					m.rotateLeft(sib)
					sib = leftOf(parentOf(x))
				}
				setColor(sib, colorOf(parentOf(x)))
				setColor(parentOf(x), BLACK)
				setColor(leftOf(sib), BLACK)
				m.rotateRight(parentOf(x))
				x = m.root
			}
		}
	}
	setColor(x, BLACK)
}

func keyOf[K, V comparable](e *Entry[K, V]) (K, bool) {
	if e != nil {
		return e.key, true
	}
	var zero K
	return zero, false
}

func colorOf[K, V comparable](p *Entry[K, V]) Color {
	if p != nil {
		return p.color
	}
	return BLACK
}

func parentOf[K, V comparable](p *Entry[K, V]) *Entry[K, V] {
	if p != nil {
		return p.parent
	}
	return nil
}

func setColor[K, V comparable](p *Entry[K, V], color Color) {
	if p != nil {
		p.color = color
	}
}

func leftOf[K, V comparable](p *Entry[K, V]) *Entry[K, V] {
	if p != nil {
		return p.left
	}
	return nil
}

func rightOf[K, V comparable](p *Entry[K, V]) *Entry[K, V] {
	if p != nil {
		return p.right
	}
	return nil
}

// Returns the successor of the specified Entry, or null if no such.
func successor[K, V comparable](t *Entry[K, V]) *Entry[K, V] {
	if t == nil {
		return nil
	} else if t.right != nil {
		var p = t.right
		for p.left != nil {
			p = p.left
		}
		return p
	} else {
		var p = t.parent
		var ch = t
		for p != nil && ch == p.right {
			ch = p
			p = p.parent
		}
		return p
	}
}

// Returns the predecessor of the specified Entry, or null if no such.
func predecessor[K, V comparable](t *Entry[K, V]) *Entry[K, V] {
	if t == nil {
		return nil
	} else if t.left != nil {
		var p = t.left
		for p.right != nil {
			p = p.right
		}
		return p
	} else {
		var p = t.parent
		var ch = t
		for p != nil && ch == p.left {
			ch = p
			p = p.parent
		}
		return p
	}
}
