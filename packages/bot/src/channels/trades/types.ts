import type { ExchangeCode, ITrade, MarketId } from "@temchik/types";

export type TradeEvent = {
  exchangeCode: ExchangeCode;
  marketId: MarketId;
  isDemoMarket: boolean;
  symbol: string;
  trade: ITrade;
};
