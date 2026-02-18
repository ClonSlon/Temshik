import { z } from "zod";

import { ZDcaBot } from '@temchik/db';

export const ZCreateDcaBotInputSchema = z.object({
  exchangeAccountId: z.number(),
  data: ZDcaBot.pick({
    name: true,
    symbol: true,
    settings: true,
  }),
});

export type TCreateDcaBotInputSchema = z.infer<typeof ZCreateDcaBotInputSchema>;
