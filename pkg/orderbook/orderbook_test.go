package orderbook

import (
	"fmt"
	"testing"
)

func TestOrderBook_LimitOrderMatching(t *testing.T) {
	ob := NewOrderBook()

	// Place a Sell Limit Order: 100 shares @ $150
	sellOrder := NewOrder("sell-1", Sell, LimitOrder, 15000, 100)
	trades1 := ob.Process(sellOrder)

	if len(trades1) != 0 {
		t.Errorf("expected 0 trades, got %d", len(trades1))
	}
	if len(ob.asks) != 1 {
		t.Errorf("expected 1 ask level, got %d", len(ob.asks))
	}
	if ob.asks[0].Price != 15000 {
		t.Errorf("expected ask price level to be 15000, got %d", ob.asks[0].Price)
	}

	// Place a Buy Limit Order: 100 shares @ $150 (exactly crosses)
	buyOrder := NewOrder("buy-1", Buy, LimitOrder, 15000, 100)
	trades2 := ob.Process(buyOrder)

	if len(trades2) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades2))
	}

	trade := trades2[0]
	if trade.MakerOrderID != "sell-1" || trade.TakerOrderID != "buy-1" {
		t.Errorf("invalid trade order IDs: maker=%s, taker=%s", trade.MakerOrderID, trade.TakerOrderID)
	}
	if trade.Price != 15000 || trade.Quantity != 100 {
		t.Errorf("invalid trade values: price=%d, qty=%d", trade.Price, trade.Quantity)
	}

	if len(ob.asks) != 0 {
		t.Errorf("expected 0 ask levels remaining, got %d", len(ob.asks))
	}
}

func TestOrderBook_MarketOrderMatching(t *testing.T) {
	ob := NewOrderBook()

	// Setup some sell depth
	ob.Process(NewOrder("sell-1", Sell, LimitOrder, 100, 50))
	ob.Process(NewOrder("sell-2", Sell, LimitOrder, 101, 50))

	// Buy Market Order: 80 shares
	marketBuy := NewOrder("buy-m", Buy, MarketOrder, 0, 80)
	trades := ob.Process(marketBuy)

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	// First trade should be 50 @ 100
	if trades[0].MakerOrderID != "sell-1" || trades[0].Price != 100 || trades[0].Quantity != 50 {
		t.Errorf("first trade mismatch: %+v", trades[0])
	}
	// Second trade should be 30 @ 101
	if trades[1].MakerOrderID != "sell-2" || trades[1].Price != 101 || trades[1].Quantity != 30 {
		t.Errorf("second trade mismatch: %+v", trades[1])
	}

	if len(ob.asks) != 1 {
		t.Errorf("expected 1 remaining ask level, got %d", len(ob.asks))
	}
	if ob.asks[0].TotalVolume != 20 {
		t.Errorf("expected remaining ask volume to be 20, got %d", ob.asks[0].TotalVolume)
	}
}

func TestOrderBook_CancelOrder(t *testing.T) {
	ob := NewOrderBook()

	o1 := NewOrder("o-1", Buy, LimitOrder, 500, 10)
	o2 := NewOrder("o-2", Buy, LimitOrder, 500, 20)

	ob.Process(o1)
	ob.Process(o2)

	if ob.bids[0].TotalVolume != 30 {
		t.Errorf("expected total volume 30, got %d", ob.bids[0].TotalVolume)
	}

	// Cancel the first order
	success := ob.Cancel("o-1")
	if !success {
		t.Error("expected cancel to succeed")
	}

	if ob.bids[0].TotalVolume != 20 {
		t.Errorf("expected total volume 20 after cancel, got %d", ob.bids[0].TotalVolume)
	}

	// Cancel second order
	ob.Cancel("o-2")
	if len(ob.bids) != 0 {
		t.Errorf("expected bids slice to be empty, got %d levels", len(ob.bids))
	}
}

func TestOrderBook_PriorityFIFO(t *testing.T) {
	ob := NewOrderBook()

	// Insert three buy orders at same price
	ob.Process(NewOrder("buy-1", Buy, LimitOrder, 100, 10))
	ob.Process(NewOrder("buy-2", Buy, LimitOrder, 100, 10))
	ob.Process(NewOrder("buy-3", Buy, LimitOrder, 100, 10))

	// Cross with a sell order of size 15
	sell := NewOrder("sell-1", Sell, LimitOrder, 100, 15)
	trades := ob.Process(sell)

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	if trades[0].MakerOrderID != "buy-1" || trades[0].Quantity != 10 {
		t.Errorf("expected first trade to fully fill buy-1, got %+v", trades[0])
	}
	if trades[1].MakerOrderID != "buy-2" || trades[1].Quantity != 5 {
		t.Errorf("expected second trade to partially fill buy-2, got %+v", trades[1])
	}
}

func BenchmarkOrderBook_Matching(b *testing.B) {
	ob := NewOrderBook()

	// Pre-fill one side of the book
	for i := 0; i < 1000; i++ {
		ob.Process(NewOrder(fmt.Sprintf("sell-%d", i), Sell, LimitOrder, uint64(1000+i), 100))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Place matching buy order
		buyOrder := NewOrder("buy", Buy, LimitOrder, 1000, 100)
		ob.Process(buyOrder)

		// Re-insert the sell order if filled, to maintain benchmark state
		if buyOrder.RemainingQty() == 0 {
			b.StopTimer()
			ob.Process(NewOrder("sell-re", Sell, LimitOrder, 1000, 100))
			b.StartTimer()
		}
	}
}
