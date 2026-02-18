import type { ExchangeCode, ITicker, MarketId } from "@temchik/types";

export type TickerEvent = {
  exchangeCode: ExchangeCode;
  marketId: MarketId;
  isDemoMarket: boolean;
  symbol: string;
  ticker: ITicker;
};
