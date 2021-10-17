package support

import (
	"context"
	"fmt"
	"sync"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "support"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

var zeroiw = types.IntervalWindow{}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Position         *bbgo.Position   `json:"position,omitempty"`
	LastBasePosition fixedpoint.Value `json:"lastBasePosition"`
}

type Strategy struct {
	*bbgo.Notifiability `json:"-"`
	*bbgo.Persistence
	*bbgo.Graceful `json:"-"`

	Symbol string       `json:"symbol"`
	Market types.Market `json:"-"`

	// Interval for checking support
	Interval types.Interval `json:"interval"`

	// moving average window for checking support (support should be under the moving average line)
	MovingAverageWindow int `json:"movingAverageWindow"`

	// LongTermMovingAverage is the second moving average line for checking support position
	LongTermMovingAverage types.IntervalWindow `json:"longTermMovingAverage"`

	Quantity              fixedpoint.Value                `json:"quantity"`
	MinVolume             fixedpoint.Value                `json:"minVolume"`
	Sensitivity           fixedpoint.Value                `json:"sensitivity"`
	TakerBuyRatio         fixedpoint.Value                `json:"takerBuyRatio"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
	Targets               []Target                        `json:"targets"`

	ResistanceStop *ResistanceStop `json:"resistanceStop"`

	PercentageTargetStop *PercentageTargetStop `json:"percentageTargetStop"`

	TrailingStop *TrailingStop `json:"trailingStop"`



	// Min BaseAsset balance to keep
	MinBaseAssetBalance fixedpoint.Value `json:"minBaseAssetBalance"`
	// Max BaseAsset balance to buy
	MaxBaseAssetBalance  fixedpoint.Value `json:"maxBaseAssetBalance"`
	MinQuoteAssetBalance fixedpoint.Value `json:"minQuoteAssetBalance"`

	ScaleQuantity *bbgo.PriceVolumeScale `json:"scaleQuantity"`

	tradeCollector            *bbgo.TradeCollector
	sentOrders, closingOrders *bbgo.OrderStore
	state                     *State

	triggerEMA  *indicator.EWMA
	longTermEMA *indicator.EWMA
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.Quantity == 0 && s.ScaleQuantity == nil {
		return fmt.Errorf("quantity or scaleQuantity can not be zero")
	}

	if s.MinVolume == 0 && s.Sensitivity == 0 {
		return fmt.Errorf("either minVolume nor sensitivity can not be zero")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: string(s.Interval)})

	if s.LongTermMovingAverage != zeroiw {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: string(s.LongTermMovingAverage.Interval)})
	}
}

func (s *Strategy) SaveState() error {
	if err := s.Persistence.Save(s.state, ID, s.Symbol, stateKey); err != nil {
		return err
	} else {
		log.Infof("state is saved => %+v", s.state)
	}
	return nil
}

func (s *Strategy) LoadState() error {
	var state State

	// load position
	if err := s.Persistence.Load(&state, ID, s.Symbol, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = &State{}
	} else {
		s.state = &state
		log.Infof("state is restored: %+v", s.state)
	}

	if s.state.Position == nil {
		s.state.Position = bbgo.NewPositionFromMarket(s.Market)
	}

	return nil
}

func (s *Strategy) submitOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, orderForms ...types.SubmitOrder) ([]types.Order, error) {
	for _, o := range orderForms {
		s.Notifiability.Notify(o)
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, orderForms...)
	if err != nil {
		return createdOrders, err
	}

	s.sentOrders.Add(createdOrders...)
	return createdOrders, nil
}

func (s *Strategy) calculateQuantity(session *bbgo.ExchangeSession, side types.SideType, closePrice fixedpoint.Value, volume float64) (fixedpoint.Value, error) {
	var quantity fixedpoint.Value
	if s.Quantity > 0 {
		quantity = s.Quantity
	} else if s.ScaleQuantity != nil {
		qf, err := s.ScaleQuantity.Scale(closePrice.Float64(), volume)
		if err != nil {
			return 0, err
		}
		quantity = fixedpoint.NewFromFloat(qf)
	}

	baseBalance, _ := session.Account.Balance(s.Market.BaseCurrency)
	if side == types.SideTypeSell {
		// quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		if s.MinBaseAssetBalance > 0 && (baseBalance.Total()-quantity) < s.MinBaseAssetBalance {
			quota := baseBalance.Available - s.MinBaseAssetBalance
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		}

	} else if side == types.SideTypeBuy {
		if s.MaxBaseAssetBalance > 0 && baseBalance.Total()+quantity > s.MaxBaseAssetBalance {
			quota := s.MaxBaseAssetBalance - baseBalance.Total()
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		}

		quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
		if !ok {
			return 0, fmt.Errorf("quote balance %s not found", s.Market.QuoteCurrency)
		}

		// for spot, we need to modify the quantity according to the quote balance
		if !session.Margin {
			// add 0.3% for price slippage
			notional := closePrice.Mul(quantity).MulFloat64(1.003)

			if s.MinQuoteAssetBalance > 0 && quoteBalance.Available-notional < s.MinQuoteAssetBalance {
				log.Warnf("modifying quantity %f according to the min quote asset balance %f %s",
					quantity.Float64(),
					quoteBalance.Available.Float64(),
					s.Market.QuoteCurrency)
				quota := quoteBalance.Available - s.MinQuoteAssetBalance
				quantity = bbgo.AdjustQuantityByMinAmount(quantity, closePrice, quota)
			} else if notional > quoteBalance.Available {
				log.Warnf("modifying quantity %f according to the quote asset balance %f %s",
					quantity.Float64(),
					quoteBalance.Available.Float64(),
					s.Market.QuoteCurrency)
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quoteBalance.Available)
			}
		}
	}

	return quantity, nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// set default values
	if s.Interval == "" {
		s.Interval = types.Interval5m
	}

	if s.MovingAverageWindow == 0 {
		s.MovingAverageWindow = 99
	}

	if s.Sensitivity > 0 {
		volRange, err := s.ScaleQuantity.ByVolumeRule.Range()
		if err != nil {
			return err
		}

		s.MinVolume = fixedpoint.NewFromFloat(volRange[0]) + fixedpoint.NewFromFloat(volRange[1]-volRange[0]).Mul(fixedpoint.NewFromFloat(1.0)-s.Sensitivity)
		log.Infof("adjusted minimal support volume to %f according to sensitivity %f", s.MinVolume.Float64(), s.Sensitivity.Float64())
	}

	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", s.Symbol)
	}
	s.Market = market

	standardIndicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	if s.LongTermMovingAverage != zeroiw {
		s.longTermEMA = standardIndicatorSet.EWMA(s.LongTermMovingAverage)
	}

	s.sentOrders = bbgo.NewOrderStore(s.Symbol)
	s.sentOrders.BindStream(session.UserDataStream)

	s.closingOrders = bbgo.NewOrderStore(s.Symbol)
	s.closingOrders.RemoveCancelled = true
	s.closingOrders.RemoveFilled = true
	s.closingOrders.BindStream(session.UserDataStream)

	s.triggerEMA = standardIndicatorSet.EWMA(types.IntervalWindow{Interval: s.Interval, Window: s.MovingAverageWindow})

	if err := s.LoadState(); err != nil {
		return err
	} else {
		s.Notify("%s state is restored => %+v", s.Symbol, s.state)
	}

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.state.Position, s.sentOrders)
	s.tradeCollector.OnPositionUpdate(func(position *bbgo.Position) {
		s.Notifiability.Notify(position)
	})
	s.tradeCollector.OnProfit(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		s.Notifiability.Notify(profit)
	})
	s.tradeCollector.OnTrade(func(trade types.Trade) {
		s.Notifiability.Notify(trade)
	})
	s.tradeCollector.BindStream(session.UserDataStream)

	s.state.LastBasePosition = s.state.Position.Base

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}
		if kline.Interval != s.Interval {
			return
		}

		// Update the target orders if the submit orders are filled
		// if all orders are filled, it means the position is updated.
		if s.state.Position.Base != s.state.LastBasePosition {
			if orders := s.closingOrders.Orders(); len(orders) > 0 {
				if err := session.Exchange.CancelOrders(ctx, orders...) ; err != nil {
					log.WithError(err).Error("failed to cancel closing orders")
				}
			}

			if s.PercentageTargetStop != nil {
				targetOrderForms := s.PercentageTargetStop.GenerateOrders(nil, nil, s.Market, s.state.Position)
				createdOrders, err := s.submitOrders(ctx, orderExecutor, targetOrderForms...)
				if err != nil {
					log.WithError(err).Error("submit profit target order error")
				} else {
					s.closingOrders.Add(createdOrders...)
				}
			}
		}

		closePriceF := kline.GetClose()
		closePrice := fixedpoint.NewFromFloat(closePriceF)

		// check support volume
		if kline.Volume < s.MinVolume.Float64() {
			return
		}

		// check taker buy ratio, we need strong buy taker
		if s.TakerBuyRatio > 0 {
			takerBuyRatio := kline.TakerBuyBaseAssetVolume / kline.Volume
			takerBuyBaseVolumeThreshold := kline.Volume * s.TakerBuyRatio.Float64()
			if takerBuyRatio < s.TakerBuyRatio.Float64() {
				s.Notify("%s: taker buy base volume %f (volume ratio %f) is less than %f (volume ratio %f)",
					s.Symbol,
					kline.TakerBuyBaseAssetVolume,
					takerBuyRatio,
					takerBuyBaseVolumeThreshold,
					kline.Volume,
					s.TakerBuyRatio.Float64(),
					kline,
				)
				return
			}
		}

		if s.longTermEMA != nil && closePriceF > s.longTermEMA.Last() {
			s.Notify("%s: closed price is above the long term moving average line %f, skipping this support",
				s.Symbol,
				s.longTermEMA.Last(),
				kline,
			)
			return
		}

		s.Notify("Found %s support: the close price %f is under EMA %f and volume %f > minimum volume %f",
			s.Symbol,
			closePrice.Float64(),
			s.triggerEMA.Last(),
			kline.Volume,
			s.MinVolume.Float64(),
			kline)

		quantity, err := s.calculateQuantity(session, types.SideTypeBuy, closePrice, kline.Volume)
		if err != nil {
			log.WithError(err).Errorf("%s quantity calculation error", s.Symbol)
			return
		}

		orderForm := types.SubmitOrder{
			Symbol:           s.Symbol,
			Market:           market,
			Side:             types.SideTypeBuy,
			Type:             types.OrderTypeMarket,
			Quantity:         quantity.Float64(),
			MarginSideEffect: s.MarginOrderSideEffect,
		}

		s.Notify("Submitting %s market order buy with quantity %f according to the base volume %f, taker buy base volume %f",
			s.Symbol,
			quantity.Float64(),
			kline.Volume,
			kline.TakerBuyBaseAssetVolume,
			orderForm)

		if _, err := s.submitOrders(ctx, orderExecutor, orderForm); err != nil {
			log.WithError(err).Error("submit order error")
			return
		}
	})

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			s.Notify("%s position is saved", s.Symbol, s.state.Position)
		}
	})

	return nil
}
