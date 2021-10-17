package support

import (
	"context"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// DynamicStopOrder maybe triggered from kline events or other events
// It needs to operate the exchange session methods to submit orders (if needed)
type DynamicStopOrder interface {
	GenerateOrders(ctx context.Context, session *bbgo.ExchangeSession, market types.Market, pos *bbgo.Position) []types.SubmitOrder
}


type Target struct {
	ProfitPercentage      float64                         `json:"profitPercentage"`
	QuantityPercentage    float64                         `json:"quantityPercentage"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

// PercentageTargetStop is a kind of stop order by setting fixed percentage target
type PercentageTargetStop struct {
	Targets []Target `json:"targets"`
}

// Subscribe implements bbgo.ExchangeSessionSubscriber interface
func (stop *PercentageTargetStop) Subscribe(session *bbgo.ExchangeSession) {
}

// GenerateOrders generates the orders from the given targets
func (stop *PercentageTargetStop) GenerateOrders(ctx context.Context, session *bbgo.ExchangeSession, market types.Market, pos *bbgo.Position) []types.SubmitOrder {
	var price = pos.AverageCost
	var quantity = pos.Base

	// submit target orders
	var targetOrders []types.SubmitOrder
	for _, target := range stop.Targets {
		targetPrice := price.Float64() * (1.0 + target.ProfitPercentage)
		targetQuantity := quantity.Float64() * target.QuantityPercentage
		targetQuoteQuantity := targetPrice * targetQuantity

		if targetQuoteQuantity <= market.MinNotional {
			continue
		}

		if targetQuantity <= market.MinQuantity {
			continue
		}

		targetOrders = append(targetOrders, types.SubmitOrder{
			Symbol:           market.Symbol,
			Market:           market,
			Type:             types.OrderTypeLimit,
			Side:             types.SideTypeSell,
			Price:            targetPrice,
			Quantity:         targetQuantity,
			MarginSideEffect: target.MarginOrderSideEffect,
			TimeInForce:      "GTC",
		})
	}

	return targetOrders
}

type TrailingStop struct {
	CallbackRate fixedpoint.Value `json:"callbackRate"`

	// ActivationPricePercentage defines the percentage of the activation price of your position average cost
	// The actual activation price will be:
	//
	//    ActivationPrice = AverageCost * (1 + ActivationPricePercentage)
	//
	ActivationPricePercentage fixedpoint.Value `json:"activationPricePercentage"`
}

// Subscribe implements bbgo.ExchangeSessionSubscriber interface
func (stop *TrailingStop) Subscribe(session *bbgo.ExchangeSession) {

}

func (stop *TrailingStop) GenerateOrders(ctx context.Context, session *bbgo.ExchangeSession, market types.Market, pos *bbgo.Position) []types.SubmitOrder {
	var price = pos.AverageCost
	var quantity = pos.Base
	_ = price
	_ = quantity
	return nil
}


// ResistanceStop is a kind of stop order by detecting resistance
type ResistanceStop struct {
	Interval      types.Interval   `json:"interval"`
	sensitivity   fixedpoint.Value `json:"sensitivity"`
	MinVolume     fixedpoint.Value `json:"minVolume"`
	TakerBuyRatio fixedpoint.Value `json:"takerBuyRatio"`
}
