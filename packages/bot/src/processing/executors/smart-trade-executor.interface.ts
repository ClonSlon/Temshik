import type { OrderWithSmartTrade, SmartTradeWithOrders } from "@temchik/db";
import { ITicker } from "@temchik/types";

export type SmartTradeContext = {
  ticker?: ITicker;
};

export interface ISmartTradeExecutor {
  smartTrade: SmartTradeWithOrders;

  /**
   * Execute the next step in the smart trade, e.g. place an order.
   * Return `true` if the step was executed, `false` otherwise.
   */
  next: (ctx?: SmartTradeContext) => Promise<boolean>;

  /**
   * Cancel all orders linked to the smart trade.
   * Return number of cancelled orders.
   */
  cancelOrders: () => Promise<number>;

  get status(): "Entering" | "Exiting" | "Finished";

  onOrderFilled?: (order: OrderWithSmartTrade) => Promise<boolean>;
  onTicker?: (ticker: ITicker) => Promise<void>;
}
