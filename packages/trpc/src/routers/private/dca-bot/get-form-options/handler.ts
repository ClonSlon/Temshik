import { exchangeProvider } from "@temchik/exchanges";
import { decomposeSymbolId } from "@temchik/tools";
import type { Context } from "../../../../utils/context.js";
import type { TGetDcaBotFormOptionsInputSchema } from "./schema.js";

type Options = {
  ctx: {
    user: NonNullable<Context["user"]>;
  };
  input: TGetDcaBotFormOptionsInputSchema;
};

export async function getFormOptions({ input }: Options) {
  const { symbolId, isDemoAccount } = input;
  const { exchangeCode, currencyPairSymbol } = decomposeSymbolId(symbolId);

  const exchange = exchangeProvider.fromCode(exchangeCode, isDemoAccount);

  const { price } = await exchange.getMarketPrice({
    symbol: currencyPairSymbol,
  });

  return {
    price,
  };
}
