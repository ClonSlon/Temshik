import type { OrderWithSmartTrade } from "@temchik/db";
import type { ExchangeCode, IWatchOrder } from "@temchik/types";

export type OrderEventType = "onFilled" | "onCanceled" | "onPlaced";

export type Subscription = {
  event: OrderEventType;
  callback: (
    exchangeOrder: IWatchOrder,
    order: OrderWithSmartTrade,
    exchangeCode: ExchangeCode,
    isDemoMarket: boolean,
  ) => void;
};
