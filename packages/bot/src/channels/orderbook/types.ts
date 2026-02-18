import type { ExchangeCode, IOrderbook, MarketId } from "@temchik/types";

export type OrderbookEvent = {
  exchangeCode: ExchangeCode;
  marketId: MarketId;
  isDemoMarket: boolean;
  symbol: string;
  orderbook: IOrderbook;
};
