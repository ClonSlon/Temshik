import { logger } from "@temchik/logger";
import { CommandResult } from "../types.js";
import { getPid } from "../utils/pid.js";

export async function status(): Promise<CommandResult> {
  const pid = getPid();

  if (pid) {
    logger.info(`Status: 🟢 Running [PID: ${pid}]`);
  } else {
    logger.info("Status: 🔴 Stopped");
  }

  return {
    result: undefined,
  };
}
