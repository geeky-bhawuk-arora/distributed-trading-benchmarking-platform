package orderbook

import "time"

// Side represents either Buy or Sell.
type Side int

const (
	Buy Side = iota
	Sell
)

func (s Side) String() string {
	if s == Buy {
		return "BUY"
	}
	return "SELL"
}

// OrderType represents either a Limit or Market order.
type OrderType int

const (
	LimitOrder OrderType = iota
	MarketOrder
)

// Order represents a single order in the system.
type Order struct {
	ID        string    `json:"id"`
	Side      Side      `json:"side"`
	Type      OrderType `json:"type"`
	Price     uint64    `json:"price"` // Scaled integer, e.g., cents or fixed-point
	Quantity  uint64    `json:"quantity"`
	FilledQty uint64    `json:"filled_quantity"`
	Timestamp time.Time `json:"timestamp"`

	// Double-linked list pointers for queueing within a Limit level
	Next  *Order `json:"-"`
	Prev  *Order `json:"-"`
	Limit *Limit `json:"-"`
}

// NewOrder creates a new order instance.
func NewOrder(id string, side Side, orderType OrderType, price uint64, qty uint64) *Order {
	return &Order{
		ID:        id,
		Side:      side,
		Type:      orderType,
		Price:     price,
		Quantity:  qty,
		Timestamp: time.Now(),
	}
}

// RemainingQty returns the unfilled quantity of the order.
func (o *Order) RemainingQty() uint64 {
	return o.Quantity - o.FilledQty
}
