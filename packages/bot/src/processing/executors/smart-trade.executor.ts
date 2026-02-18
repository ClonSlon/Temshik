import type { ExchangeAccountWithCredentials, SmartTradeWithOrders } from "@temchik/db";
import { xprisma } from "@temchik/db";
import { exchangeProvider } from "@temchik/exchanges";
import { XSmartTradeType } from "@temchik/types";
import type { ISmartTradeExecutor } from "./smart-trade-executor.interface.js";
import { TradeExecutor } from "./trade/trade.executor.js";
import { ArbExecutor } from "./arb/arb.executor.js";
import { DcaExecutor } from "./dca/dca.executor.js";

/**
 * Combine all type of SmartTrades into one executor.
 */
export class SmartTradeExecutor {
  static create(
    smartTrade: SmartTradeWithOrders,
    exchangeAccount: ExchangeAccountWithCredentials,
  ): ISmartTradeExecutor {
    const exchange = exchangeProvider.fromAccount(exchangeAccount);

    switch (smartTrade.type as XSmartTradeType) {
      case "Trade":
        return new TradeExecutor(smartTrade, exchange);
      case "ARB":
        return new ArbExecutor(smartTrade);
      case "DCA":
        return new DcaExecutor(smartTrade, exchange);
      default:
        throw new Error(`Unknown SmartTrade type: ${smartTrade.type}`);
    }
  }

  static async fromId(id: number): Promise<ISmartTradeExecutor> {
    const smartTrade = await xprisma.smartTrade.findUniqueOrThrow({
      where: {
        id,
      },
      include: {
        orders: true,
        exchangeAccount: true,
      },
    });
    const exchange = exchangeProvider.fromAccount(smartTrade.exchangeAccount);

    switch (smartTrade.type as XSmartTradeType) {
      case "Trade":
        return new TradeExecutor(smartTrade, exchange);
      case "ARB":
        return new ArbExecutor(smartTrade);
      case "DCA":
        return new DcaExecutor(smartTrade, exchange);
      default:
        throw new Error(`Unknown SmartTrade type: ${smartTrade.type}`);
    }
  }
}
