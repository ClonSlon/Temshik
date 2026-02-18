import type { Trade } from '@temchik/bot-processor';
import { OrderSideEnum } from "@temchik/types";
import type { ActiveOrder } from "../types/index.js";

export function buyOrder(smartTrade: Trade): ActiveOrder {
  return {
    side: OrderSideEnum.Buy,
    quantity: smartTrade.entryOrder.quantity,
    price: (smartTrade.entryOrder.filledPrice || smartTrade.entryOrder.price)!,
  };
}
