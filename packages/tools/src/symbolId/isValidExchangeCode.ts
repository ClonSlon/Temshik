import { ExchangeCode } from "@temchik/types";

export function isValidExchangeCode(exchangeCode: ExchangeCode) {
  return Object.keys(ExchangeCode).includes(exchangeCode);
}
