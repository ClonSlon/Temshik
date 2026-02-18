import { exchangeProvider } from "@temchik/exchanges";
import { decomposeSymbolId } from "@temchik/tools";
import type { Context } from "../../../../utils/context.js";
import type { TGetSymbolInputSchema } from "./schema.js";

type Options = {
  ctx: Context;
  input: TGetSymbolInputSchema;
};

export async function getSymbol(opts: Options) {
  const { input } = opts;

  const { exchangeCode, currencyPairSymbol } = decomposeSymbolId(input.symbolId);
  const exchangeService = exchangeProvider.fromCode(exchangeCode, input.isDemoAccount);

  const symbol = await exchangeService.getSymbol({
    currencyPair: currencyPairSymbol,
  });

  return symbol;
}
