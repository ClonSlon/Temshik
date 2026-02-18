import type { BarSize, ExchangeCode, ICandlestick, MarketId } from "@temchik/types";

export type CandleEvent = {
  exchangeCode: ExchangeCode;
  symbol: string;
  marketId: MarketId;
  isDemoMarket: boolean;
  timeframe: BarSize;
  /**
   * Last closed candle
   */
  candle: ICandlestick;
  /**
   * Candles history
   */
  history: ICandlestick[];
};
