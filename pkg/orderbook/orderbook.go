package orderbook

import (
	"sort"
)

// Trade represents an execution match between two orders.
type Trade struct {
	MakerOrderID string `json:"maker_order_id"`
	TakerOrderID string `json:"taker_order_id"`
	Price        uint64 `json:"price"`
	Quantity     uint64 `json:"quantity"`
}

// PriceLevel represents simplified order book depth info for APIs.
type PriceLevel struct {
	Price  uint64 `json:"price"`
	Volume uint64 `json:"volume"`
}

// OrderBook matches bids and asks based on price-time priority.
type OrderBook struct {
	bids   []*Limit          // Sorted descending (highest buy price first)
	asks   []*Limit          // Sorted ascending (lowest sell price first)
	orders map[string]*Order // O(1) lookup map of all active orders in the book
}

// NewOrderBook creates a new, empty order book.
func NewOrderBook() *OrderBook {
	return &OrderBook{
		bids:   make([]*Limit, 0),
		asks:   make([]*Limit, 0),
		orders: make(map[string]*Order),
	}
}

// Process processes a incoming order. It matches it against the opposite book side,
// returns any trades generated, and adds any unfilled portion to the book (if Limit order).
func (ob *OrderBook) Process(order *Order) []Trade {
	var trades []Trade

	if order.Quantity == 0 {
		return trades
	}

	if order.Side == Buy {
		trades = ob.matchOrder(order, &ob.asks)
		if order.RemainingQty() > 0 && order.Type == LimitOrder {
			ob.addLimitOrder(order, &ob.bids, true)
		}
	} else {
		trades = ob.matchOrder(order, &ob.bids)
		if order.RemainingQty() > 0 && order.Type == LimitOrder {
			ob.addLimitOrder(order, &ob.asks, false)
		}
	}

	return trades
}

// Cancel removes an active order from the book. Returns true if cancelled successfully.
func (ob *OrderBook) Cancel(orderID string) bool {
	order, exists := ob.orders[orderID]
	if !exists {
		return false
	}

	limit := order.Limit
	if limit == nil {
		return false
	}

	limit.RemoveOrder(order)
	delete(ob.orders, order.ID)

	// Clean up empty limit levels
	if limit.TotalVolume == 0 {
		if order.Side == Buy {
			ob.removeLimitLevel(&ob.bids, limit.Price, true)
		} else {
			ob.removeLimitLevel(&ob.asks, limit.Price, false)
		}
	}

	return true
}

// GetDepth returns the L2 orderbook snapshot (up to specified depth).
func (ob *OrderBook) GetDepth(depth int) ([]PriceLevel, []PriceLevel) {
	bidsDepth := make([]PriceLevel, 0, depth)
	asksDepth := make([]PriceLevel, 0, depth)

	for i := 0; i < len(ob.bids) && i < depth; i++ {
		bidsDepth = append(bidsDepth, PriceLevel{
			Price:  ob.bids[i].Price,
			Volume: ob.bids[i].TotalVolume,
		})
	}

	for i := 0; i < len(ob.asks) && i < depth; i++ {
		asksDepth = append(asksDepth, PriceLevel{
			Price:  ob.asks[i].Price,
			Volume: ob.asks[i].TotalVolume,
		})
	}

	return bidsDepth, asksDepth
}

// matchOrder matches an incoming order against the opposite side's price levels.
func (ob *OrderBook) matchOrder(takerOrder *Order, makerLimits *[]*Limit) []Trade {
	var trades []Trade

	for len(*makerLimits) > 0 && takerOrder.RemainingQty() > 0 {
		bestLimit := (*makerLimits)[0]

		// For Limit orders, check if crossing is possible.
		if takerOrder.Type == LimitOrder {
			if takerOrder.Side == Buy && takerOrder.Price < bestLimit.Price {
				break
			}
			if takerOrder.Side == Sell && takerOrder.Price > bestLimit.Price {
				break
			}
		}

		// Match against orders at this limit level
		for bestLimit.Head != nil && takerOrder.RemainingQty() > 0 {
			makerOrder := bestLimit.Head
			matchQty := min(takerOrder.RemainingQty(), makerOrder.RemainingQty())

			// Fill matching quantities
			takerOrder.FilledQty += matchQty
			makerOrder.FilledQty += matchQty
			bestLimit.TotalVolume -= matchQty

			trades = append(trades, Trade{
				MakerOrderID: makerOrder.ID,
				TakerOrderID: takerOrder.ID,
				Price:        bestLimit.Price,
				Quantity:     matchQty,
			})

			if makerOrder.RemainingQty() == 0 {
				bestLimit.RemoveOrder(makerOrder)
				delete(ob.orders, makerOrder.ID)
			}
		}

		// Remove the limit level if it has been fully exhausted
		if bestLimit.TotalVolume == 0 {
			*makerLimits = (*makerLimits)[1:]
		}
	}

	return trades
}

// addLimitOrder inserts a limit order into the sorted slice of limit levels.
func (ob *OrderBook) addLimitOrder(order *Order, limits *[]*Limit, descending bool) {
	price := order.Price

	// Binary search to find if price level already exists
	idx := sort.Search(len(*limits), func(i int) bool {
		if descending {
			return (*limits)[i].Price <= price
		}
		return (*limits)[i].Price >= price
	})

	var limit *Limit
	if idx < len(*limits) && (*limits)[idx].Price == price {
		limit = (*limits)[idx]
	} else {
		// Create new limit level and insert it at idx to keep sorted order
		limit = NewLimit(price)
		*limits = append(*limits, nil)
		copy((*limits)[idx+1:], (*limits)[idx:])
		(*limits)[idx] = limit
	}

	limit.AddOrder(order)
	ob.orders[order.ID] = order
}

// removeLimitLevel removes an empty limit level from the sorted slice.
func (ob *OrderBook) removeLimitLevel(limits *[]*Limit, price uint64, descending bool) {
	idx := sort.Search(len(*limits), func(i int) bool {
		if descending {
			return (*limits)[i].Price <= price
		}
		return (*limits)[i].Price >= price
	})

	if idx < len(*limits) && (*limits)[idx].Price == price {
		*limits = append((*limits)[:idx], (*limits)[idx+1:]...)
	}
}

// min is a helper function to get the minimum of two uint64 values.
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
