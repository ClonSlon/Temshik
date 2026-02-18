import type { OrderStatus, XOrderStatus } from "@temchik/types";

/**
 * Map Exchange order status to DB order status
 * @param status - Order status
 */
export function toDbStatus(status: OrderStatus): XOrderStatus {
  switch (status) {
    case "open":
      return "Placed";
    case "partially_filled":
      return "Placed";
    case "filled":
      return "Filled";
    case "canceled":
      return "Canceled";
  }
}
