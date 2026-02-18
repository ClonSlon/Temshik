# tRPC Router

> Legacy TypeScript package. The Go backend now serves the live tRPC API; this package is retained for reference only.

## Setup

### Backend (with Next.js)

```ts
// src/app/api/trpc/[trpc]/route.ts
import { appRouter, createContext } from "@temchik/trpc";
import { fetchRequestHandler } from "@trpc/server/adapters/fetch";

const handler = (req: Request) =>
  fetchRequestHandler({
    endpoint: "/api/trpc",
    req,
    router: appRouter,
    createContext: () => ({
      //...
    }),
  });

export { handler as GET, handler as POST };
```

### Backend (with Nest.js)

```ts
// src/trpc.middleware.ts
import { INestApplication, Injectable } from "@nestjs/common";
import * as trpcExpress from "@trpc/server/adapters/express";

import { appRouter } from "@temchik/trpc";

@Injectable()
export class TrpcMiddleware {
  applyMiddleware(app: INestApplication) {
    app.use(
      `/api/trpc`,
      trpcExpress.createExpressMiddleware({
        router: appRouter,
        createContext: () => ({
          // ...
        }),
      }),
    );
  }
}

// main.ts
import { TrpcMiddleware } from "src/trpc.middleware";

async function bootstrap() {
  // ...
  const trpc = app.get(TrpcMiddleware);
  trpc.applyMiddleware(app);
  // ...
}
bootstrap();
```

## Usage

### Client

```ts
// lib/trc/client.ts
import { createTRPCProxyClient, httpBatchLink } from "@trpc/client";
import type { AppRouter } from "@temchik/trpc";
//     👆 **type-only** import

export const tClient = createTRPCProxyClient<AppRouter>({
  links: [
    httpBatchLink({
      url: `${getBaseUrl()}/api/trpc`,
    }),
  ],
});
```

### Client with React

```tsx
// App.ts
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

export const tClient = createTRPCReact<AppRouter>({});

export const TrpcProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [queryClient] = useState(() => new QueryClient());
  const [trpcClient] = useState(() =>
    tClient.createClient({
      links: [
        httpBatchLink({
          url: `${getBaseUrl()}/api/trpc`,
        }),
      ],
    }),
  );
  return (
    <trpcClient.Provider client={trpcClient} queryClient={queryClient}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </trpcClient.Provider>
  );
};
```

### Server side (Next.js)

```tsx
// lib/trpc/server.ts
import { appRouter } from "@temchik/trpc";

export const tServer = appRouter.createCaller({
  /* context here */
});

// usage inside a Server Compoent
import { tServer } from "src/lib/trpc/server";

export default async function Page() {
  const data = await tServer.exchangeAccount.list();

  return <div>{JSON.stringify(data)}</div>;
}
```
