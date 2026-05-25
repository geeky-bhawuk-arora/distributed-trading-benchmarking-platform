package orderbook

// Limit represents a single price level in the order book.
// It maintains a doubly-linked list of orders at this price level
// to ensure FIFO (First-In, First-Out) time priority.
type Limit struct {
	Price       uint64 // The price level
	TotalVolume uint64 // Total quantity of unfilled shares/contracts at this price
	Head        *Order // First order in the queue (oldest, highest priority)
	Tail        *Order // Last order in the queue (newest, lowest priority)
}

// NewLimit creates a new Limit level.
func NewLimit(price uint64) *Limit {
	return &Limit{
		Price: price,
	}
}

// AddOrder appends an order to the tail of the limit queue.
func (l *Limit) AddOrder(order *Order) {
	order.Limit = l
	if l.Head == nil {
		l.Head = order
		l.Tail = order
	} else {
		l.Tail.Next = order
		order.Prev = l.Tail
		order.Next = nil
		l.Tail = order
	}
	l.TotalVolume += order.RemainingQty()
}

// RemoveOrder removes a specific order from the limit queue.
func (l *Limit) RemoveOrder(order *Order) {
	if order.Prev != nil {
		order.Prev.Next = order.Next
	} else {
		// order is the Head of the list
		l.Head = order.Next
	}

	if order.Next != nil {
		order.Next.Prev = order.Prev
	} else {
		// order is the Tail of the list
		l.Tail = order.Prev
	}

	l.TotalVolume -= order.RemainingQty()
	order.Limit = nil
	order.Next = nil
	order.Prev = nil
}
