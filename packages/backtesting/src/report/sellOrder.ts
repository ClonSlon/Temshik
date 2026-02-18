import { Trade } from '@temchik/bot-processor';
import { OrderSideEnum } from "@temchik/types";
import type { ActiveOrder } from "../types/index.js";

export function sellOrder(smartTrade: Trade): ActiveOrder {
  return {
    side: OrderSideEnum.Sell,
    quantity: smartTrade.entryOrder.quantity,
    price: (smartTrade.tpOrder!.filledPrice || smartTrade.tpOrder!.price)!,
  };
}
